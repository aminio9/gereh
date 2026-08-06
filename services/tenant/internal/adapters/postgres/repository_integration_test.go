package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func repositoryIntegrationTest(t *testing.T) *Repository {
	t.Helper()

	databaseURL := os.Getenv("TENANT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TENANT_TEST_DATABASE_URL is not configured")
	}

	pool, err := pgxpool.New(
		context.Background(),
		databaseURL,
	)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}

	t.Cleanup(pool.Close)

	return New(pool)
}

func mustV7(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate UUIDv7: %v", err)
	}

	return id.String()
}

func cleanupTenants(
	t *testing.T,
	repository *Repository,
	tenantIDs []string,
	actorUserID string,
) {
	t.Helper()

	for _, tenantID := range tenantIDs {
		cleanupTenant(
			t,
			repository,
			tenantID,
			actorUserID,
		)
	}
}

func cleanupTenant(
	t *testing.T,
	repository *Repository,
	tenantID string,
	actorUserID string,
) {
	t.Helper()

	ctx := context.Background()

	// Outbox is operational and has no tenant RLS.
	if _, err := repository.pool.Exec(
		ctx,
		`
			DELETE FROM tenant_outbox
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	); err != nil {
		t.Errorf("cleanup tenant outbox: %v", err)
		return
	}

	transaction, err := repository.beginTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		t.Errorf(
			"begin tenant cleanup: %v",
			err,
		)
		return
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if _, err := transaction.Exec(
		ctx,
		`
			DELETE FROM tenant_tenants
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	); err != nil {
		t.Errorf("cleanup tenant: %v", err)
		return
	}

	if err := transaction.Commit(ctx); err != nil {
		t.Errorf("commit tenant cleanup: %v", err)
	}
}

func createTestTenant(
	ctx context.Context,
	t *testing.T,
	repository *Repository,
	actorUserID string,
	requestID string,
	slug string,
) (domain.CreateTenantResult, error) {
	t.Helper()

	created, err := createTestTenantRaw(
		ctx,
		t,
		repository,
		actorUserID,
		requestID,
		slug,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	if _, err := repository.MarkOnboardingRunning(
		ctx,
		ports.MarkOnboardingRunningParams{
			ServicePrincipalID: workflowServicePrincipalID,
			TenantID:           created.Context.Tenant.ID,
			OperationID:        created.Operation.ID,
			WorkflowID:         "tenant-onboarding/" + created.Operation.ID,
			WorkflowRunID:      mustV7(t),
			StartedAt:          time.Now().UTC(),
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        created.Context.Tenant.ID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: time.Now().UTC(),
			},
		},
	); err != nil {
		return domain.CreateTenantResult{}, err
	}

	return repository.CompleteOnboarding(
		ctx,
		ports.CompleteOnboardingParams{
			ServicePrincipalID: workflowServicePrincipalID,
			TenantID:           created.Context.Tenant.ID,
			OperationID:        created.Operation.ID,
			CompletedAt:        time.Now().UTC(),
			Event: func(
				_ domain.TenantContext,
			) (domain.OutboxEvent, error) {
				return domain.OutboxEvent{
					ID:         mustV7(t),
					Topic:      "gereh.tenant.events.v1",
					Key:        created.Context.Tenant.ID,
					Envelope:   []byte{0x0a, 0x00},
					OccurredAt: time.Now().UTC(),
				}, nil
			},
		},
	)
}

func createTestTenantRaw(
	ctx context.Context,
	t *testing.T,
	repository *Repository,
	actorUserID string,
	requestID string,
	slug string,
) (domain.CreateTenantResult, error) {
	t.Helper()

	tenantID, err := uuid.NewV7()
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	now := time.Now().UTC()

	contextValue := domain.TenantContext{
		Tenant: domain.Tenant{
			ID:              tenantID.String(),
			Slug:            slug,
			DisplayName:     "Integration Tenant",
			Status:          domain.StatusProvisioning,
			Region:          "local",
			RetentionDays:   90,
			Version:         1,
			CreatedByUserID: actorUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		Membership: domain.Membership{
			TenantID:  tenantID.String(),
			UserID:    actorUserID,
			Role:      domain.RoleOwner,
			Version:   1,
			CreatedBy: actorUserID,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Entitlements: domain.Entitlements{
			TenantID:  tenantID.String(),
			PlanKey:   "free",
			Features:  map[string]bool{"agent_coordination": true},
			Limits:    map[string]int64{"members": 5, "agents": 10, "projects": 3},
			Version:   1,
			UpdatedAt: now,
		},
	}

	operationID, err := uuid.NewV7()
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	operation := domain.Operation{
		ID:           operationID.String(),
		TenantID:     tenantID.String(),
		ActorUserID:  actorUserID,
		RequestID:    requestID,
		State:        domain.OperationStatePending,
		ResourceName: "tenants/" + tenantID.String(),
		Metadata: map[string]string{
			"kind":   "tenant_onboarding",
			"region": "local",
		},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return repository.CreateTenant(
		ctx,
		ports.CreateTenantParams{
			Context:   contextValue,
			Operation: operation,
			RequestID: requestID,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        tenantID.String(),
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
}

func addTestMember(
	ctx context.Context,
	t *testing.T,
	repository *Repository,
	actorUserID string,
	tenantID string,
	userID string,
	role domain.Role,
	expectedTenantVersion int64,
) (domain.Membership, int64, error) {
	now := time.Now().UTC()

	membership := domain.Membership{
		TenantID:  tenantID,
		UserID:    userID,
		Role:      role,
		Version:   1,
		CreatedBy: actorUserID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	newVersion := expectedTenantVersion + 1

	result, err := repository.AddMember(
		ctx,
		ports.AddMemberParams{
			ActorUserID:           actorUserID,
			Membership:            membership,
			ExpectedTenantVersion: expectedTenantVersion,
			NewTenantVersion:      newVersion,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        tenantID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
	if err != nil {
		return domain.Membership{}, 0, err
	}

	return result, newVersion, nil
}

func outboxCount(
	ctx context.Context,
	repository *Repository,
	tenantID string,
) (int64, error) {
	var count int64

	err := repository.pool.QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM tenant_outbox
			WHERE partition_key = $1
		`,
		tenantID,
	).Scan(&count)

	return count, err
}

func TestCreateTenantCreatesExactlyOneOwner(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actor := mustV7(t)

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		actor,
		"create-owner-test",
		fmt.Sprintf("owner-test-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{result.Context.Tenant.ID}, actor)
	})

	if result.Context.Membership.Role != domain.RoleOwner {
		t.Fatalf(
			"creator role = %q, want owner",
			result.Context.Membership.Role,
		)
	}

	if result.Context.Membership.UserID != actor {
		t.Fatalf(
			"owner user = %q, want %q",
			result.Context.Membership.UserID,
			actor,
		)
	}
}

func TestRepeatedCreationWithSameRequestIDReturnsSameTenant(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actor := mustV7(t)
	requestID := "idempotent-request"

	first, err := createTestTenant(
		ctx,
		t,
		repository,
		actor,
		requestID,
		fmt.Sprintf("idem-test-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{first.Context.Tenant.ID}, actor)
	})

	second, err := createTestTenant(
		ctx,
		t,
		repository,
		actor,
		requestID,
		fmt.Sprintf("idem-test-%d-2", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}

	if second.Context.Tenant.ID != first.Context.Tenant.ID {
		t.Fatalf(
			"idempotent tenant ID changed: %q != %q",
			second.Context.Tenant.ID,
			first.Context.Tenant.ID,
		)
	}
}

func TestDuplicateSlugReturnsConflict(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actor := mustV7(t)
	slug := fmt.Sprintf("dup-slug-%d", time.Now().UTC().UnixNano())

	first, err := createTestTenant(
		ctx,
		t,
		repository,
		actor,
		"duplicate-slug-request-1",
		slug,
	)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{first.Context.Tenant.ID}, actor)
	})

	_, err = createTestTenant(
		ctx,
		t,
		repository,
		actor,
		"duplicate-slug-request-2",
		slug,
	)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestStaleTenantVersionReturnsVersionConflict(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actor := mustV7(t)

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		actor,
		"stale-version-test",
		fmt.Sprintf("stale-version-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{result.Context.Tenant.ID}, actor)
	})

	now := time.Now().UTC()
	updated := result.Context.Tenant
	updated.DisplayName = "Updated"
	updated.Version = result.Context.Tenant.Version + 1
	updated.UpdatedAt = now

	_, err = repository.UpdateTenant(
		ctx,
		ports.UpdateTenantParams{
			ActorUserID:     actor,
			Tenant:          updated,
			ExpectedVersion: 99,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        result.Context.Tenant.ID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

func TestAdminCannotPromoteToOwner(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	owner := mustV7(t)
	admin := mustV7(t)
	member := mustV7(t)

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		owner,
		"admin-promote-test",
		fmt.Sprintf("admin-promote-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{result.Context.Tenant.ID}, owner)
	})

	tenantVersion := result.Context.Tenant.Version

	_, tenantVersion, err = addTestMember(
		ctx,
		t,
		repository,
		owner,
		result.Context.Tenant.ID,
		admin,
		domain.RoleAdmin,
		tenantVersion,
	)
	if err != nil {
		t.Fatalf("add admin: %v", err)
	}

	_, tenantVersion, err = addTestMember(
		ctx,
		t,
		repository,
		owner,
		result.Context.Tenant.ID,
		member,
		domain.RoleMember,
		tenantVersion,
	)
	if err != nil {
		t.Fatalf("add member: %v", err)
	}

	target, err := repository.GetMembership(
		ctx,
		result.Context.Tenant.ID,
		member,
	)
	if err != nil {
		t.Fatalf("get target membership: %v", err)
	}

	now := time.Now().UTC()
	promoted := target
	promoted.Role = domain.RoleOwner
	promoted.Version = target.Version + 1
	promoted.UpdatedAt = now

	_, err = repository.UpdateMemberRole(
		ctx,
		ports.UpdateMemberRoleParams{
			ActorUserID:               admin,
			Membership:                promoted,
			PreviousRole:              target.Role,
			ExpectedMembershipVersion: target.Version,
			ExpectedTenantVersion:     tenantVersion,
			NewTenantVersion:          tenantVersion + 1,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        result.Context.Tenant.ID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestAdminCannotDemoteOrRemoveOwner(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	owner := mustV7(t)
	admin := mustV7(t)

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		owner,
		"admin-owner-test",
		fmt.Sprintf("admin-owner-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{result.Context.Tenant.ID}, owner)
	})

	tenantVersion := result.Context.Tenant.Version

	_, tenantVersion, err = addTestMember(
		ctx,
		t,
		repository,
		owner,
		result.Context.Tenant.ID,
		admin,
		domain.RoleAdmin,
		tenantVersion,
	)
	if err != nil {
		t.Fatalf("add admin: %v", err)
	}

	ownerMembership, err := repository.GetMembership(
		ctx,
		result.Context.Tenant.ID,
		owner,
	)
	if err != nil {
		t.Fatalf("get owner membership: %v", err)
	}

	now := time.Now().UTC()
	demoted := ownerMembership
	demoted.Role = domain.RoleMember
	demoted.Version = ownerMembership.Version + 1
	demoted.UpdatedAt = now

	_, err = repository.UpdateMemberRole(
		ctx,
		ports.UpdateMemberRoleParams{
			ActorUserID:               admin,
			Membership:                demoted,
			PreviousRole:              ownerMembership.Role,
			ExpectedMembershipVersion: ownerMembership.Version,
			ExpectedTenantVersion:     tenantVersion,
			NewTenantVersion:          tenantVersion + 1,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        result.Context.Tenant.ID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden on demotion, got %v", err)
	}

	err = repository.RemoveMember(
		ctx,
		ports.RemoveMemberParams{
			ActorUserID:               admin,
			TenantID:                  result.Context.Tenant.ID,
			UserID:                    owner,
			PreviousRole:              ownerMembership.Role,
			ExpectedMembershipVersion: ownerMembership.Version,
			ExpectedTenantVersion:     tenantVersion,
			NewTenantVersion:          tenantVersion + 1,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        result.Context.Tenant.ID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden on removal, got %v", err)
	}
}

func TestFinalOwnerCannotBeRemoved(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	owner := mustV7(t)

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		owner,
		"last-owner-test",
		fmt.Sprintf("last-owner-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{result.Context.Tenant.ID}, owner)
	})

	ownerMembership, err := repository.GetMembership(
		ctx,
		result.Context.Tenant.ID,
		owner,
	)
	if err != nil {
		t.Fatalf("get owner membership: %v", err)
	}

	now := time.Now().UTC()

	err = repository.RemoveMember(
		ctx,
		ports.RemoveMemberParams{
			ActorUserID:               owner,
			TenantID:                  result.Context.Tenant.ID,
			UserID:                    owner,
			PreviousRole:              ownerMembership.Role,
			ExpectedMembershipVersion: ownerMembership.Version,
			ExpectedTenantVersion:     result.Context.Tenant.Version,
			NewTenantVersion:          result.Context.Tenant.Version + 1,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        result.Context.Tenant.ID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
	if !errors.Is(err, domain.ErrLastOwner) {
		t.Fatalf("expected ErrLastOwner, got %v", err)
	}
}

func TestConcurrentOwnerDemotionsOnlyOneSucceeds(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	ownerA := mustV7(t)
	ownerB := mustV7(t)

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		ownerA,
		"concurrent-demote-test",
		fmt.Sprintf("concurrent-demote-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{result.Context.Tenant.ID}, ownerA)
	})

	tenantVersion := result.Context.Tenant.Version

	_, tenantVersion, err = addTestMember(
		ctx,
		t,
		repository,
		ownerA,
		result.Context.Tenant.ID,
		ownerB,
		domain.RoleOwner,
		tenantVersion,
	)
	if err != nil {
		t.Fatalf("add second owner: %v", err)
	}

	var waitGroup sync.WaitGroup
	results := make(chan error, 2)

	demote := func(actor string, target string) {
		defer waitGroup.Done()

		membership, err := repository.GetMembership(
			ctx,
			result.Context.Tenant.ID,
			target,
		)
		if err != nil {
			results <- err
			return
		}

		now := time.Now().UTC()
		demoted := membership
		demoted.Role = domain.RoleAdmin
		demoted.Version = membership.Version + 1
		demoted.UpdatedAt = now

		_, err = repository.UpdateMemberRole(
			ctx,
			ports.UpdateMemberRoleParams{
				ActorUserID:               actor,
				Membership:                demoted,
				PreviousRole:              membership.Role,
				ExpectedMembershipVersion: membership.Version,
				ExpectedTenantVersion:     tenantVersion,
				NewTenantVersion:          tenantVersion + 1,
				Event: domain.OutboxEvent{
					ID:         mustV7(t),
					Topic:      "gereh.tenant.events.v1",
					Key:        result.Context.Tenant.ID,
					Envelope:   []byte{0x0a, 0x00},
					OccurredAt: now,
				},
			},
		)
		results <- err
	}

	waitGroup.Add(2)
	go demote(ownerA, ownerB)
	go demote(ownerB, ownerA)
	waitGroup.Wait()
	close(results)

	successes := 0
	var receivedErr error

	for err := range results {
		if err == nil {
			successes++
			continue
		}

		if errors.Is(err, domain.ErrVersionConflict) ||
			errors.Is(err, domain.ErrLastOwner) {
			receivedErr = err
			continue
		}

		t.Fatalf("unexpected demotion error: %v", err)
	}

	if successes != 1 {
		t.Fatalf(
			"expected exactly one successful demotion, got %d (second error: %v)",
			successes,
			receivedErr,
		)
	}
}

func TestSuccessfulMutationInsertsOneOutboxRow(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actor := mustV7(t)

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		actor,
		"outbox-count-test",
		fmt.Sprintf("outbox-count-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{result.Context.Tenant.ID}, actor)
	})

	count, err := outboxCount(ctx, repository, result.Context.Tenant.ID)
	if err != nil {
		t.Fatalf("count outbox: %v", err)
	}

	if count != 3 {
		t.Fatalf("creation outbox rows = %d, want 3", count)
	}

	now := time.Now().UTC()
	updated := result.Context.Tenant
	updated.DisplayName = "Renamed"
	updated.Version = result.Context.Tenant.Version + 1
	updated.UpdatedAt = now

	_, err = repository.UpdateTenant(
		ctx,
		ports.UpdateTenantParams{
			ActorUserID:     actor,
			Tenant:          updated,
			ExpectedVersion: result.Context.Tenant.Version,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        result.Context.Tenant.ID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
	if err != nil {
		t.Fatalf("update tenant: %v", err)
	}

	count, err = outboxCount(ctx, repository, result.Context.Tenant.ID)
	if err != nil {
		t.Fatalf("count outbox after update: %v", err)
	}

	if count != 4 {
		t.Fatalf("update outbox rows = %d, want 4", count)
	}
}

func TestFailedMutationInsertsNoOutboxRow(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actor := mustV7(t)

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		actor,
		"failed-outbox-test",
		fmt.Sprintf("failed-outbox-%d", time.Now().UTC().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(t, repository, []string{result.Context.Tenant.ID}, actor)
	})

	count, err := outboxCount(ctx, repository, result.Context.Tenant.ID)
	if err != nil {
		t.Fatalf("count outbox: %v", err)
	}

	if count != 3 {
		t.Fatalf("creation outbox rows = %d, want 3", count)
	}

	now := time.Now().UTC()
	stale := result.Context.Tenant
	stale.DisplayName = "Stale"
	stale.Version = result.Context.Tenant.Version + 1
	stale.UpdatedAt = now

	_, err = repository.UpdateTenant(
		ctx,
		ports.UpdateTenantParams{
			ActorUserID:     actor,
			Tenant:          stale,
			ExpectedVersion: 999,
			Event: domain.OutboxEvent{
				ID:         mustV7(t),
				Topic:      "gereh.tenant.events.v1",
				Key:        result.Context.Tenant.ID,
				Envelope:   []byte{0x0a, 0x00},
				OccurredAt: now,
			},
		},
	)
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}

	count, err = outboxCount(ctx, repository, result.Context.Tenant.ID)
	if err != nil {
		t.Fatalf("count outbox after failed update: %v", err)
	}

	if count != 3 {
		t.Fatalf(
			"failed mutation outbox rows = %d, want unchanged 3",
			count,
		)
	}
}
