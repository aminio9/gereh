package postgres

import (
	"context"
	"testing"
	"time"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/model-access/internal/application"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type staticTestCatalog struct{}

var _ ports.StaticCatalogLoader = staticTestCatalog{}

func (staticTestCatalog) LoadPlatformOfferings(_ string, _ string) ([]domain.DiscoveredModel, error) {
	return []domain.DiscoveredModel{
		{
			ProviderModelID:     "gpt-4o",
			DisplayName:         "GPT-4o",
			AgentUsable:         true,
			Capabilities:        []string{"chat", "tools"},
			InputModalities:     []string{"text", "image"},
			OutputModalities:    []string{"text"},
			ContextWindowTokens: 128000,
			MaxOutputTokens:     4096,
		},
		{
			ProviderModelID:     "text-embedding-3-small",
			DisplayName:         "Text Embedding 3 Small",
			AgentUsable:         false,
			Capabilities:        []string{"embeddings"},
			InputModalities:     []string{"text"},
			OutputModalities:    []string{"embeddings"},
			ContextWindowTokens: 8191,
		},
	}, nil
}

func (staticTestCatalog) FindCuratedMetadata(_ string, _ string) *domain.DiscoveredModel {
	return nil
}

type agentDirectoryFake struct {
	agent ports.AgentReference
	err   error
}

func (f agentDirectoryFake) GetAgent(_ context.Context, _, _, _ string) (ports.AgentReference, error) {
	if f.err != nil {
		return ports.AgentReference{}, f.err
	}
	return f.agent, nil
}

func TestCatalogRefreshAndOfferingsLifecycle(t *testing.T) {
	test := newModelAccessIntegrationTest(t)
	ctx := context.Background()

	// 1. Create a platform-managed connection
	connection, err := test.service.CreateConnection(
		ctx,
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypePlatformManaged,
			DisplayName:    "Test OpenAI Platform",
		},
	)
	require.NoError(t, err)
	require.Equal(t, domain.ConnectionStatusActive, connection.Status)

	// 2. Wire service with static catalog loader
	serviceWithCatalog, err := application.New(
		test.repository,
		allowAllAuthorizer{},
		test.secretStore,
		stubIntegrationVerifier{},
		integrationFingerprinter(t),
		application.Config{
			EventTopic:     "gereh.model.events.v1",
			IdempotencyTTL: 24 * time.Hour,
		},
		application.WithStaticCatalog(staticTestCatalog{}),
	)
	require.NoError(t, err)

	// 3. Claim the catalog refresh job enqueued upon active connection creation
	jobs, err := test.repository.ClaimCatalogRefresh(ctx, 10, 30*time.Second)
	require.NoError(t, err)
	require.NotEmpty(t, jobs)

	var targetJob *domain.CatalogRefreshJob
	for _, j := range jobs {
		if j.ConnectionID == connection.ID {
			targetJob = &j
			break
		}
	}
	require.NotNil(t, targetJob)

	// 4. Execute catalog refresh
	err = serviceWithCatalog.ExecuteCatalogRefresh(ctx, *targetJob)
	require.NoError(t, err)

	// 5. Query refreshed status
	refresh, err := serviceWithCatalog.GetModelCatalogRefresh(
		ctx,
		test.userA,
		test.tenantA,
		targetJob.RefreshID,
	)
	require.NoError(t, err)
	require.Equal(t, domain.CatalogRefreshStatus("succeeded"), refresh.Status)
	require.Equal(t, 2, refresh.DiscoveredCount)
	require.Equal(t, 2, refresh.AvailableCount)
	require.Equal(t, 0, refresh.UnavailableCount)

	// 6. List offerings
	listRes, err := serviceWithCatalog.ListModelOfferings(
		ctx,
		application.ListOfferingsInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			ConnectionID:    connection.ID,
			AgentUsableOnly: false,
			Limit:           10,
		},
	)
	require.NoError(t, err)
	require.Len(t, listRes.Offerings, 2)

	var gpt4oOffering *domain.ModelOffering
	for _, off := range listRes.Offerings {
		if off.ProviderModelID == "gpt-4o" {
			gpt4oOffering = &off
			break
		}
	}
	require.NotNil(t, gpt4oOffering)
	require.True(t, gpt4oOffering.AgentUsable)
	require.Equal(t, domain.OfferingStatusAvailable, gpt4oOffering.Status)

	// 7. Test agent binding with agentDirectoryFake
	agentID := uuid.NewString()
	companyID := uuid.NewString()

	serviceWithAgent, err := application.New(
		test.repository,
		allowAllAuthorizer{},
		test.secretStore,
		stubIntegrationVerifier{},
		integrationFingerprinter(t),
		application.Config{
			EventTopic:     "gereh.model.events.v1",
			IdempotencyTTL: 24 * time.Hour,
		},
		application.WithAgentDirectory(agentDirectoryFake{
			agent: ports.AgentReference{
				TenantID:  test.tenantA,
				CompanyID: companyID,
				AgentID:   agentID,
				Status:    organizationv1.AgentStatus_AGENT_STATUS_READY,
				Version:   1,
			},
		}),
	)
	require.NoError(t, err)

	// Bind agent
	bindingKey := uuid.NewString()
	binding, err := serviceWithAgent.SetAgentModelBinding(
		ctx,
		application.SetAgentBindingInput{
			ActorUserID:       test.userA,
			TenantID:          test.tenantA,
			AgentID:           agentID,
			IdempotencyKey:    bindingKey,
			ExpectedVersion:   0,
			PrimaryOfferingID: gpt4oOffering.ID,
			FallbackPolicy:    domain.FallbackPolicyNone,
		},
	)
	require.NoError(t, err)
	require.Equal(t, domain.BindingStatusActive, binding.Status)
	require.Equal(t, gpt4oOffering.ID, binding.PrimaryOfferingID)
	require.EqualValues(t, 1, binding.Version)

	// Query binding
	queriedBinding, err := serviceWithAgent.GetAgentModelBinding(
		ctx,
		test.userA,
		test.tenantA,
		agentID,
	)
	require.NoError(t, err)
	require.Equal(t, binding.AgentID, queriedBinding.AgentID)
	require.Equal(t, binding.PrimaryOfferingID, queriedBinding.PrimaryOfferingID)

	// Remove binding
	removeKey := uuid.NewString()
	removedBinding, err := serviceWithAgent.RemoveAgentModelBinding(
		ctx,
		application.RemoveAgentBindingInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			AgentID:         agentID,
			IdempotencyKey:  removeKey,
			ExpectedVersion: 1,
		},
	)
	require.NoError(t, err)
	require.Equal(t, domain.BindingStatusRemoved, removedBinding.Status)
	require.EqualValues(t, 2, removedBinding.Version)
}
