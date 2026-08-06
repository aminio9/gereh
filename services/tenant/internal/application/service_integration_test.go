package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	tenantpostgres "github.com/aminio9/gereh/services/tenant/internal/adapters/postgres"
	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func applicationIntegrationService(
	t *testing.T,
) (*Service, *pgxpool.Pool) {
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

	service, err := New(
		tenantpostgres.New(pool),
		Config{
			EventTopic:                 "gereh.tenant.events.v1",
			DefaultRegion:              "local",
			AllowedRegions:             []string{"local"},
			DefaultRetentionDays:       90,
			WorkflowServicePrincipalID: "018f7767-28d2-7f5c-a693-0bb4c8ee4ae0",
		},
	)
	if err != nil {
		t.Fatalf("create application service: %v", err)
	}

	return service, pool
}

func applicationTestActor(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate actor UUID: %v", err)
	}

	return id.String()
}

func applicationActivateTenant(
	t *testing.T,
	service *Service,
	created domain.CreateTenantResult,
) domain.CreateTenantResult {
	t.Helper()

	ctx := context.Background()

	if _, err := service.MarkOnboardingRunning(
		ctx,
		MarkOnboardingRunningInput{
			TenantID:      created.Context.Tenant.ID,
			OperationID:   created.Operation.ID,
			WorkflowID:    "tenant-onboarding/" + created.Operation.ID,
			WorkflowRunID: uuid.NewString(),
		},
	); err != nil {
		t.Fatalf("mark onboarding running: %v", err)
	}

	activated, err := service.CompleteOnboarding(
		ctx,
		CompleteOnboardingInput{
			TenantID:    created.Context.Tenant.ID,
			OperationID: created.Operation.ID,
		},
	)
	if err != nil {
		t.Fatalf("complete onboarding: %v", err)
	}

	return activated
}

func applicationOutboxCount(
	t *testing.T,
	pool *pgxpool.Pool,
	tenantID string,
) int64 {
	t.Helper()

	var count int64

	err := pool.QueryRow(
		context.Background(),
		`
			SELECT count(*)
			FROM tenant_outbox
			WHERE partition_key = $1
		`,
		tenantID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}

	return count
}

func applicationCleanupTenant(
	t *testing.T,
	pool *pgxpool.Pool,
	tenantID string,
) {
	t.Helper()

	_, err := pool.Exec(
		context.Background(),
		`
			DELETE FROM tenant_outbox
			WHERE partition_key = $1
		`,
		tenantID,
	)
	if err != nil {
		t.Errorf("cleanup outbox rows: %v", err)
	}

	_, err = pool.Exec(
		context.Background(),
		`
			DELETE FROM tenant_tenants
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	)
	if err != nil {
		t.Errorf("cleanup tenant: %v", err)
	}
}

func TestOwnerReceivesEveryPermission(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "owner-permissions-request",
			Slug:        fmt.Sprintf("owner-perm-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Owner Permissions",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	permissions := []domain.Permission{
		domain.PermissionTenantRead,
		domain.PermissionTenantUpdate,
		domain.PermissionTenantArchive,
		domain.PermissionMemberList,
		domain.PermissionMemberAdd,
		domain.PermissionMemberUpdateRole,
		domain.PermissionMemberRemove,
		domain.PermissionEntitlementRead,
	}

	for _, permission := range permissions {
		decision, err := service.CheckAuthorization(
			ctx,
			owner,
			created.Context.Tenant.ID,
			permission,
		)
		if err != nil {
			t.Fatalf(
				"check permission %q: %v",
				permission,
				err,
			)
		}

		if !decision.Allowed {
			t.Fatalf(
				"owner permission %q was denied",
				permission,
			)
		}
	}
}

func TestAdminCannotArchiveTenant(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	admin := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "admin-archive-request",
			Slug:        fmt.Sprintf("admin-archive-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Admin Archive",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      admin,
			Role:        domain.RoleAdmin,
		},
	)
	if err != nil {
		t.Fatalf("add admin: %v", err)
	}

	decision, err := service.CheckAuthorization(
		ctx,
		admin,
		created.Context.Tenant.ID,
		domain.PermissionTenantArchive,
	)
	if err != nil {
		t.Fatalf("check archive permission: %v", err)
	}

	if decision.Allowed {
		t.Fatal("admin archive permission was unexpectedly granted")
	}
}

func TestAdminCannotAssignOwner(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	admin := applicationTestActor(t)
	member := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "admin-assign-owner-request",
			Slug:        fmt.Sprintf("admin-assign-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Admin Assign",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      admin,
			Role:        domain.RoleAdmin,
		},
	)
	if err != nil {
		t.Fatalf("add admin: %v", err)
	}

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: admin,
			TenantID:    created.Context.Tenant.ID,
			UserID:      member,
			Role:        domain.RoleOwner,
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden assigning owner, got %v", err)
	}
}

func TestAdminCannotModifyOrRemoveOwner(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	admin := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "admin-owner-request",
			Slug:        fmt.Sprintf("admin-owner-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Admin Owner",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      admin,
			Role:        domain.RoleAdmin,
		},
	)
	if err != nil {
		t.Fatalf("add admin: %v", err)
	}

	ownerMembership, err := service.GetTenantContext(
		ctx,
		owner,
		created.Context.Tenant.ID,
	)
	if err != nil {
		t.Fatalf("get owner context: %v", err)
	}

	_, _, err = service.UpdateMemberRole(
		ctx,
		UpdateMemberRoleInput{
			ActorUserID:               admin,
			TenantID:                  created.Context.Tenant.ID,
			UserID:                    owner,
			Role:                      domain.RoleMember,
			ExpectedMembershipVersion: ownerMembership.Membership.Version,
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden demoting owner, got %v", err)
	}

	_, err = service.RemoveMember(
		ctx,
		RemoveMemberInput{
			ActorUserID:               admin,
			TenantID:                  created.Context.Tenant.ID,
			UserID:                    owner,
			ExpectedMembershipVersion: ownerMembership.Membership.Version,
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden removing owner, got %v", err)
	}
}

func TestMemberCanListButCannotMutateMembership(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	member := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "member-list-request",
			Slug:        fmt.Sprintf("member-list-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Member List",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      member,
			Role:        domain.RoleMember,
		},
	)
	if err != nil {
		t.Fatalf("add member: %v", err)
	}

	decision, err := service.CheckAuthorization(
		ctx,
		member,
		created.Context.Tenant.ID,
		domain.PermissionMemberList,
	)
	if err != nil {
		t.Fatalf("check member list permission: %v", err)
	}

	if !decision.Allowed {
		t.Fatal("member list permission was denied")
	}

	_, _, err = service.ListMembers(
		ctx,
		member,
		created.Context.Tenant.ID,
		10,
		"",
	)
	if err != nil {
		t.Fatalf("member could not list members: %v", err)
	}

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: member,
			TenantID:    created.Context.Tenant.ID,
			UserID:      applicationTestActor(t),
			Role:        domain.RoleMember,
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden adding member, got %v", err)
	}
}

func TestViewerCannotListMembers(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	viewer := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "viewer-list-request",
			Slug:        fmt.Sprintf("viewer-list-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Viewer List",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      viewer,
			Role:        domain.RoleViewer,
		},
	)
	if err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	_, _, err = service.ListMembers(
		ctx,
		viewer,
		created.Context.Tenant.ID,
		10,
		"",
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden listing members, got %v", err)
	}

	decision, err := service.CheckAuthorization(
		ctx,
		viewer,
		created.Context.Tenant.ID,
		domain.PermissionMemberList,
	)
	if err != nil {
		t.Fatalf("check viewer member list permission: %v", err)
	}

	if decision.Allowed {
		t.Fatal("viewer member list permission was unexpectedly granted")
	}
}

func TestViewerCanReadTenantAndEntitlements(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	viewer := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "viewer-read-request",
			Slug:        fmt.Sprintf("viewer-read-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Viewer Read",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      viewer,
			Role:        domain.RoleViewer,
		},
	)
	if err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	for _, permission := range []domain.Permission{
		domain.PermissionTenantRead,
		domain.PermissionEntitlementRead,
	} {
		decision, err := service.CheckAuthorization(
			ctx,
			viewer,
			created.Context.Tenant.ID,
			permission,
		)
		if err != nil {
			t.Fatalf(
				"check viewer permission %q: %v",
				permission,
				err,
			)
		}

		if !decision.Allowed {
			t.Fatalf(
				"viewer permission %q was denied",
				permission,
			)
		}
	}
}

func TestNonMemberReceivesDeniedDecisionWithoutInternalDetails(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	stranger := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "stranger-decision-request",
			Slug:        fmt.Sprintf("stranger-decision-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Stranger Decision",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	decision, err := service.CheckAuthorization(
		ctx,
		stranger,
		created.Context.Tenant.ID,
		domain.PermissionTenantRead,
	)
	if err != nil {
		t.Fatalf("check stranger authorization: %v", err)
	}

	if decision.Allowed {
		t.Fatal("stranger authorization was unexpectedly granted")
	}

	if decision.DenialReason !=
		domain.DenialReasonNotMember {
		t.Fatalf(
			"DenialReason = %q, want %q",
			decision.DenialReason,
			domain.DenialReasonNotMember,
		)
	}
}

func TestArchivedTenantRetainsReadAndDeniesMutation(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "archived-request",
			Slug:        fmt.Sprintf("archived-authz-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Archived Authz",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	activated := applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	archived, err := service.ArchiveTenant(
		ctx,
		owner,
		created.Context.Tenant.ID,
		activated.Context.Tenant.Version,
	)
	if err != nil {
		t.Fatalf("archive tenant: %v", err)
	}
	readDecision, err := service.CheckAuthorization(
		ctx,
		owner,
		archived.Tenant.ID,
		domain.PermissionTenantRead,
	)
	if err != nil {
		t.Fatalf("check archived read: %v", err)
	}

	if !readDecision.Allowed {
		t.Fatal("archived tenant read was denied")
	}

	updateDecision, err := service.CheckAuthorization(
		ctx,
		owner,
		archived.Tenant.ID,
		domain.PermissionTenantUpdate,
	)
	if err != nil {
		t.Fatalf("check archived update: %v", err)
	}

	if updateDecision.Allowed {
		t.Fatal("archived tenant mutation was granted")
	}

	if updateDecision.DenialReason !=
		domain.DenialReasonTenantArchived {
		t.Fatalf(
			"DenialReason = %q, want %q",
			updateDecision.DenialReason,
			domain.DenialReasonTenantArchived,
		)
	}
}

func TestStaleTenantAndMembershipVersionsFail(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	admin := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "stale-versions-request",
			Slug:        fmt.Sprintf("stale-versions-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Stale Versions",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	displayName := "Renamed"
	_, err = service.UpdateTenant(
		ctx,
		UpdateTenantInput{
			ActorUserID:     owner,
			TenantID:        created.Context.Tenant.ID,
			ExpectedVersion: created.Context.Tenant.Version + 99,
			DisplayName:     &displayName,
		},
	)
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict on tenant update, got %v", err)
	}

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      admin,
			Role:        domain.RoleAdmin,
		},
	)
	if err != nil {
		t.Fatalf("add admin: %v", err)
	}

	target, err := service.GetTenantContext(
		ctx,
		admin,
		created.Context.Tenant.ID,
	)
	if err != nil {
		t.Fatalf("get admin context: %v", err)
	}

	_, _, err = service.UpdateMemberRole(
		ctx,
		UpdateMemberRoleInput{
			ActorUserID:               owner,
			TenantID:                  created.Context.Tenant.ID,
			UserID:                    admin,
			Role:                      domain.RoleMember,
			ExpectedMembershipVersion: target.Membership.Version + 99,
		},
	)
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf(
			"expected ErrVersionConflict on membership update, got %v",
			err,
		)
	}
}

func TestFinalOwnerCannotRemoveThemself(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "final-owner-request",
			Slug:        fmt.Sprintf("final-owner-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Final Owner",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	contextValue, err := service.GetTenantContext(
		ctx,
		owner,
		created.Context.Tenant.ID,
	)
	if err != nil {
		t.Fatalf("get owner context: %v", err)
	}

	_, err = service.RemoveMember(
		ctx,
		RemoveMemberInput{
			ActorUserID:               owner,
			TenantID:                  created.Context.Tenant.ID,
			UserID:                    owner,
			ExpectedMembershipVersion: contextValue.Membership.Version,
		},
	)
	if !errors.Is(err, domain.ErrLastOwner) {
		t.Fatalf("expected ErrLastOwner, got %v", err)
	}
}

func TestMembershipMutationsWriteExactlyOneOutboxEvent(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	member := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "outbox-member-request",
			Slug:        fmt.Sprintf("outbox-member-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Outbox Member",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	if count := applicationOutboxCount(t, pool, created.Context.Tenant.ID); count != 3 {
		t.Fatalf("creation outbox rows = %d, want 3", count)
	}

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      member,
			Role:        domain.RoleMember,
		},
	)
	if err != nil {
		t.Fatalf("add member: %v", err)
	}

	if count := applicationOutboxCount(t, pool, created.Context.Tenant.ID); count != 4 {
		t.Fatalf("add member outbox rows = %d, want 4", count)
	}

	memberContext, err := service.GetTenantContext(
		ctx,
		member,
		created.Context.Tenant.ID,
	)
	if err != nil {
		t.Fatalf("get member context: %v", err)
	}

	_, _, err = service.UpdateMemberRole(
		ctx,
		UpdateMemberRoleInput{
			ActorUserID:               owner,
			TenantID:                  created.Context.Tenant.ID,
			UserID:                    member,
			Role:                      domain.RoleViewer,
			ExpectedMembershipVersion: memberContext.Membership.Version,
		},
	)
	if err != nil {
		t.Fatalf("update member role: %v", err)
	}

	if count := applicationOutboxCount(t, pool, created.Context.Tenant.ID); count != 5 {
		t.Fatalf("role change outbox rows = %d, want 5", count)
	}

	memberContext, err = service.GetTenantContext(
		ctx,
		member,
		created.Context.Tenant.ID,
	)
	if err != nil {
		t.Fatalf("get member context after role change: %v", err)
	}

	_, err = service.RemoveMember(
		ctx,
		RemoveMemberInput{
			ActorUserID:               owner,
			TenantID:                  created.Context.Tenant.ID,
			UserID:                    member,
			ExpectedMembershipVersion: memberContext.Membership.Version,
		},
	)
	if err != nil {
		t.Fatalf("remove member: %v", err)
	}

	if count := applicationOutboxCount(t, pool, created.Context.Tenant.ID); count != 6 {
		t.Fatalf("remove member outbox rows = %d, want 6", count)
	}
}

func TestFailedAuthorizationWritesNoOutboxEvent(t *testing.T) {
	service, pool := applicationIntegrationService(t)

	ctx := context.Background()
	owner := applicationTestActor(t)
	member := applicationTestActor(t)
	stranger := applicationTestActor(t)

	created, err := service.CreateTenant(
		ctx,
		CreateTenantInput{
			ActorUserID: owner,
			RequestID:   "failed-authz-request",
			Slug:        fmt.Sprintf("failed-authz-%d", time.Now().UTC().UnixNano()),
			DisplayName: "Failed Authz",
		},
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	applicationActivateTenant(t, service, created)

	t.Cleanup(func() {
		applicationCleanupTenant(t, pool, created.Context.Tenant.ID)
	})

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: owner,
			TenantID:    created.Context.Tenant.ID,
			UserID:      member,
			Role:        domain.RoleMember,
		},
	)
	if err != nil {
		t.Fatalf("add member: %v", err)
	}

	countBefore := applicationOutboxCount(t, pool, created.Context.Tenant.ID)

	_, _, err = service.AddMember(
		ctx,
		MemberInput{
			ActorUserID: member,
			TenantID:    created.Context.Tenant.ID,
			UserID:      stranger,
			Role:        domain.RoleMember,
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	displayName := "Unauthorized"
	_, err = service.UpdateTenant(
		ctx,
		UpdateTenantInput{
			ActorUserID:     member,
			TenantID:        created.Context.Tenant.ID,
			ExpectedVersion: created.Context.Tenant.Version,
			DisplayName:     &displayName,
		},
	)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden on update, got %v", err)
	}

	if count := applicationOutboxCount(t, pool, created.Context.Tenant.ID); count != countBefore {
		t.Fatalf(
			"failed authorization changed outbox rows: before %d, after %d",
			countBefore,
			count,
		)
	}
}
