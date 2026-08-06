package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/ports"
	"github.com/aminio9/gereh/services/tenant/internal/protoutil"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Config defines tenant defaults and supported regions.
type Config struct {
	EventTopic                 string
	DefaultRegion              string
	DefaultRetentionDays       int32
	AllowedRegions             []string
	WorkflowServicePrincipalID string
}

// CreateTenantInput defines tenant creation input.
type CreateTenantInput struct {
	ActorUserID   string
	RequestID     string
	Slug          string
	DisplayName   string
	Region        string
	RetentionDays int32
}

// UpdateTenantInput defines tenant update input.
type UpdateTenantInput struct {
	ActorUserID     string
	TenantID        string
	ExpectedVersion int64
	DisplayName     *string
	Region          *string
	RetentionDays   *int32
}

// MemberInput defines membership creation input.
type MemberInput struct {
	ActorUserID string
	TenantID    string
	UserID      string
	Role        domain.Role
}

// UpdateMemberRoleInput defines a role mutation.
type UpdateMemberRoleInput struct {
	ActorUserID               string
	TenantID                  string
	UserID                    string
	Role                      domain.Role
	ExpectedMembershipVersion int64
}

// RemoveMemberInput defines membership removal.
type RemoveMemberInput struct {
	ActorUserID               string
	TenantID                  string
	UserID                    string
	ExpectedMembershipVersion int64
}

// Service implements tenant use cases.
type Service struct {
	repository     ports.Repository
	config         Config
	allowedRegions map[string]struct{}
	now            func() time.Time
}

// New creates a tenant application service.
func New(
	repository ports.Repository,
	config Config,
) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf(
			"tenant repository is required",
		)
	}

	if strings.TrimSpace(config.EventTopic) == "" {
		return nil, fmt.Errorf(
			"tenant event topic is required",
		)
	}

	if err := validateUUID(
		"workflow_service_principal_id",
		config.WorkflowServicePrincipalID,
	); err != nil {
		return nil, err
	}

	if err := validateRetentionDays(
		config.DefaultRetentionDays,
	); err != nil {
		return nil, err
	}

	allowed := make(
		map[string]struct{},
		len(config.AllowedRegions),
	)

	for _, value := range config.AllowedRegions {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			allowed[value] = struct{}{}
		}
	}

	if _, ok := allowed[config.DefaultRegion]; !ok {
		return nil, fmt.Errorf(
			"default region must be in the allowed-region set",
		)
	}

	return &Service{
		repository:     repository,
		config:         config,
		allowedRegions: allowed,
		now:            time.Now,
	}, nil
}

// CreateTenant accepts a tenant creation request into the provisioning
// pipeline. It returns a tenant context and a durable operation the caller
// can poll.
func (service *Service) CreateTenant(
	ctx context.Context,
	input CreateTenantInput,
) (domain.CreateTenantResult, error) {
	if err := validateUUID(
		"actor_user_id",
		input.ActorUserID,
	); err != nil {
		return domain.CreateTenantResult{}, err
	}

	if strings.TrimSpace(input.RequestID) == "" ||
		len(input.RequestID) > 128 {
		return domain.CreateTenantResult{}, fmt.Errorf(
			"%w: request_id must contain 1-128 characters",
			domain.ErrInvalidArgument,
		)
	}

	slug, err := normalizeSlug(input.Slug)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	displayName, err := validateDisplayName(
		input.DisplayName,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	region := strings.TrimSpace(input.Region)
	if region == "" {
		region = service.config.DefaultRegion
	}

	region, err = validateRegion(
		region,
		service.allowedRegions,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	retentionDays := input.RetentionDays
	if retentionDays == 0 {
		retentionDays = service.config.DefaultRetentionDays
	}

	if err := validateRetentionDays(retentionDays); err != nil {
		return domain.CreateTenantResult{}, err
	}

	tenantID, err := uuid.NewV7()
	if err != nil {
		return domain.CreateTenantResult{}, fmt.Errorf(
			"generate tenant ID: %w",
			err,
		)
	}

	operationID, err := uuid.NewV7()
	if err != nil {
		return domain.CreateTenantResult{}, fmt.Errorf(
			"generate operation ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	contextValue := domain.TenantContext{
		Tenant: domain.Tenant{
			ID:              tenantID.String(),
			Slug:            slug,
			DisplayName:     displayName,
			Status:          domain.StatusProvisioning,
			Region:          region,
			RetentionDays:   retentionDays,
			Version:         1,
			CreatedByUserID: input.ActorUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		Membership: domain.Membership{
			TenantID:  tenantID.String(),
			UserID:    input.ActorUserID,
			Role:      domain.RoleOwner,
			Version:   1,
			CreatedBy: input.ActorUserID,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Entitlements: domain.Entitlements{
			TenantID: tenantID.String(),
			PlanKey:  "free",
			Features: map[string]bool{
				"agent_coordination": true,
			},
			Limits: map[string]int64{
				"members":  5,
				"agents":   10,
				"projects": 3,
			},
			Version:   1,
			UpdatedAt: now,
		},
	}

	operation := domain.Operation{
		ID:           operationID.String(),
		TenantID:     tenantID.String(),
		ActorUserID:  input.ActorUserID,
		RequestID:    input.RequestID,
		State:        domain.OperationStatePending,
		ResourceName: "tenants/" + tenantID.String(),
		Metadata: map[string]string{
			"slug":         slug,
			"display_name": displayName,
		},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	event, err := newTenantOutboxEvent(
		ctx,
		service.config.EventTopic,
		"tenant.created",
		tenantID.String(),
		1,
		&tenantv1.TenantCreated{
			Context:     protoutil.Context(contextValue),
			OperationId: operation.ID,
		},
		now,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	return service.repository.CreateTenant(
		ctx,
		ports.CreateTenantParams{
			Context:   contextValue,
			Operation: operation,
			RequestID: input.RequestID,
			Event:     event,
		},
	)
}

// GetTenantContext validates membership and returns trusted tenant context.
func (service *Service) GetTenantContext(
	ctx context.Context,
	actorUserID string,
	tenantID string,
) (domain.TenantContext, error) {
	if err := validateUUID(
		"actor_user_id",
		actorUserID,
	); err != nil {
		return domain.TenantContext{}, err
	}

	if err := validateUUID(
		"tenant_id",
		tenantID,
	); err != nil {
		return domain.TenantContext{}, err
	}

	contextValue, err :=
		service.repository.GetTenantContext(
			ctx,
			tenantID,
			actorUserID,
		)
	if err != nil {
		return domain.TenantContext{}, err
	}

	if err := requirePermission(
		contextValue,
		domain.PermissionTenantRead,
	); err != nil {
		return domain.TenantContext{}, err
	}

	return decorateTenantContext(contextValue), nil
}

// ListTenants lists tenants visible to the actor.
func (service *Service) ListTenants(
	ctx context.Context,
	actorUserID string,
	pageSize int32,
	pageToken string,
) ([]domain.TenantContext, string, error) {
	if err := validateUUID(
		"actor_user_id",
		actorUserID,
	); err != nil {
		return nil, "", err
	}

	cursorID, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	var cursor *ports.TenantCursor

	if cursorID != "" {
		if err := validateUUID(
			"page_token",
			cursorID,
		); err != nil {
			return nil, "", err
		}

		cursor = &ports.TenantCursor{
			TenantID: cursorID,
		}
	}

	limit := normalizePageSize(pageSize)

	contexts, err := service.repository.ListTenantContexts(
		ctx,
		actorUserID,
		limit+1,
		cursor,
	)
	if err != nil {
		return nil, "", err
	}

	nextToken := ""

	if len(contexts) > limit {
		contexts = contexts[:limit]

		nextToken = encodePageToken(
			contexts[len(contexts)-1].Tenant.ID,
		)
	}

	for index := range contexts {
		contexts[index] =
			decorateTenantContext(contexts[index])
	}

	return contexts, nextToken, nil
}

// UpdateTenant updates mutable tenant settings.
func (service *Service) UpdateTenant(
	ctx context.Context,
	input UpdateTenantInput,
) (domain.TenantContext, error) {
	current, err := service.GetTenantContext(
		ctx,
		input.ActorUserID,
		input.TenantID,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	if current.Tenant.Status != domain.StatusActive {
		return domain.TenantContext{}, domain.ErrArchived
	}

	if err := requirePermission(
		current,
		domain.PermissionTenantUpdate,
	); err != nil {
		return domain.TenantContext{}, err
	}

	if input.ExpectedVersion != current.Tenant.Version {
		return domain.TenantContext{},
			domain.ErrVersionConflict
	}

	updated := current.Tenant

	if input.DisplayName != nil {
		updated.DisplayName, err = validateDisplayName(
			*input.DisplayName,
		)
		if err != nil {
			return domain.TenantContext{}, err
		}
	}

	if input.Region != nil {
		updated.Region, err = validateRegion(
			*input.Region,
			service.allowedRegions,
		)
		if err != nil {
			return domain.TenantContext{}, err
		}
	}

	if input.RetentionDays != nil {
		if err := validateRetentionDays(
			*input.RetentionDays,
		); err != nil {
			return domain.TenantContext{}, err
		}

		updated.RetentionDays = *input.RetentionDays
	}

	if input.DisplayName == nil &&
		input.Region == nil &&
		input.RetentionDays == nil {
		return domain.TenantContext{}, fmt.Errorf(
			"%w: at least one update field is required",
			domain.ErrInvalidArgument,
		)
	}

	now := service.now().UTC()
	updated.Version++
	updated.UpdatedAt = now

	event, err := newTenantOutboxEvent(
		ctx,
		service.config.EventTopic,
		"tenant.updated",
		updated.ID,
		updated.Version,
		&tenantv1.TenantUpdated{
			Tenant:          protoutil.Tenant(updated),
			UpdatedByUserId: input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	return service.repository.UpdateTenant(
		ctx,
		ports.UpdateTenantParams{
			ActorUserID:     input.ActorUserID,
			Tenant:          updated,
			ExpectedVersion: input.ExpectedVersion,
			Event:           event,
		},
	)
}

// ArchiveTenant archives a tenant. Only owners may perform this operation.
func (service *Service) ArchiveTenant(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	expectedVersion int64,
) (domain.TenantContext, error) {
	current, err := service.GetTenantContext(
		ctx,
		actorUserID,
		tenantID,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	if !domain.RoleAllows(
		current.Membership.Role,
		domain.PermissionTenantArchive,
	) {
		return domain.TenantContext{},
			domain.ErrForbidden
	}

	if expectedVersion != current.Tenant.Version {
		return domain.TenantContext{},
			domain.ErrVersionConflict
	}

	if current.Tenant.Status == domain.StatusArchived {
		return current, nil
	}

	now := service.now().UTC()
	archived := current.Tenant
	archived.Status = domain.StatusArchived
	archived.Version++
	archived.UpdatedAt = now
	archived.ArchivedAt = &now

	event, err := newTenantOutboxEvent(
		ctx,
		service.config.EventTopic,
		"tenant.archived",
		tenantID,
		archived.Version,
		&tenantv1.TenantArchived{
			Tenant:           protoutil.Tenant(archived),
			ArchivedByUserId: actorUserID,
		},
		now,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	return service.repository.ArchiveTenant(
		ctx,
		ports.ArchiveTenantParams{
			ActorUserID:     actorUserID,
			Tenant:          archived,
			ExpectedVersion: expectedVersion,
			Event:           event,
		},
	)
}

// ListMembers lists tenant memberships.
func (service *Service) ListMembers(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	pageSize int32,
	pageToken string,
) ([]domain.Membership, string, error) {
	current, err := service.GetTenantContext(
		ctx,
		actorUserID,
		tenantID,
	)
	if err != nil {
		return nil, "", err
	}

	if err := requirePermission(
		current,
		domain.PermissionMemberList,
	); err != nil {
		return nil, "", err
	}

	cursorID, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	var cursor *ports.MemberCursor

	if cursorID != "" {
		if err := validateUUID(
			"page_token",
			cursorID,
		); err != nil {
			return nil, "", err
		}

		cursor = &ports.MemberCursor{
			UserID: cursorID,
		}
	}

	limit := normalizePageSize(pageSize)

	memberships, err := service.repository.ListMembers(
		ctx,
		tenantID,
		actorUserID,
		limit+1,
		cursor,
	)
	if err != nil {
		return nil, "", err
	}

	nextToken := ""

	if len(memberships) > limit {
		memberships = memberships[:limit]

		nextToken = encodePageToken(
			memberships[len(memberships)-1].UserID,
		)
	}

	return memberships, nextToken, nil
}

// AddMember creates a tenant membership.
func (service *Service) AddMember(
	ctx context.Context,
	input MemberInput,
) (domain.Membership, int64, error) {
	if err := validateUUID(
		"user_id",
		input.UserID,
	); err != nil {
		return domain.Membership{}, 0, err
	}

	if !domain.IsKnownRole(input.Role) {
		return domain.Membership{}, 0, fmt.Errorf(
			"%w: unsupported tenant role",
			domain.ErrInvalidArgument,
		)
	}

	current, err := service.GetTenantContext(
		ctx,
		input.ActorUserID,
		input.TenantID,
	)
	if err != nil {
		return domain.Membership{}, 0, err
	}

	if current.Tenant.Status != domain.StatusActive {
		return domain.Membership{}, 0, domain.ErrArchived
	}

	if err := requirePermission(
		current,
		domain.PermissionMemberAdd,
	); err != nil {
		return domain.Membership{}, 0, err
	}

	if !domain.CanManageMember(
		current.Membership.Role,
		"",
		input.Role,
	) {
		return domain.Membership{}, 0, domain.ErrForbidden
	}

	now := service.now().UTC()
	newTenantVersion := current.Tenant.Version + 1

	membership := domain.Membership{
		TenantID:  input.TenantID,
		UserID:    input.UserID,
		Role:      input.Role,
		Version:   1,
		CreatedBy: input.ActorUserID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	event, err := newTenantOutboxEvent(
		ctx,
		service.config.EventTopic,
		"tenant.member_added",
		input.TenantID,
		newTenantVersion,
		&tenantv1.TenantMemberAdded{
			Membership:    protoutil.Membership(membership),
			AddedByUserId: input.ActorUserID,
			TenantVersion: newTenantVersion,
		},
		now,
	)
	if err != nil {
		return domain.Membership{}, 0, err
	}

	result, err := service.repository.AddMember(
		ctx,
		ports.AddMemberParams{
			ActorUserID:           input.ActorUserID,
			Membership:            membership,
			ExpectedTenantVersion: current.Tenant.Version,
			NewTenantVersion:      newTenantVersion,
			Event:                 event,
		},
	)
	if err != nil {
		return domain.Membership{}, 0, err
	}

	return result, newTenantVersion, nil
}

// UpdateMemberRole changes/ manages a tenant membership role.
func (service *Service) UpdateMemberRole(
	ctx context.Context,
	input UpdateMemberRoleInput,
) (domain.Membership, int64, error) {
	if !domain.IsKnownRole(input.Role) {
		return domain.Membership{}, 0, fmt.Errorf(
			"%w: unsupported tenant role",
			domain.ErrInvalidArgument,
		)
	}

	current, err := service.GetTenantContext(
		ctx,
		input.ActorUserID,
		input.TenantID,
	)
	if err != nil {
		return domain.Membership{}, 0, err
	}

	target, err := service.repository.GetMembership(
		ctx,
		input.TenantID,
		input.UserID,
	)
	if err != nil {
		return domain.Membership{}, 0, err
	}

	if err := requirePermission(
		current,
		domain.PermissionMemberUpdateRole,
	); err != nil {
		return domain.Membership{}, 0, err
	}

	if !domain.CanManageMember(
		current.Membership.Role,
		target.Role,
		input.Role,
	) {
		return domain.Membership{}, 0, domain.ErrForbidden
	}

	if input.ExpectedMembershipVersion != target.Version {
		return domain.Membership{}, 0,
			domain.ErrVersionConflict
	}

	now := service.now().UTC()
	previousRole := target.Role
	target.Role = input.Role
	target.Version++
	target.UpdatedAt = now

	newTenantVersion := current.Tenant.Version + 1

	event, err := newTenantOutboxEvent(
		ctx,
		service.config.EventTopic,
		"tenant.member_role_changed",
		input.TenantID,
		newTenantVersion,
		&tenantv1.TenantMemberRoleChanged{
			Membership:      protoutil.Membership(target),
			PreviousRole:    protoutil.Role(previousRole),
			ChangedByUserId: input.ActorUserID,
			TenantVersion:   newTenantVersion,
		},
		now,
	)
	if err != nil {
		return domain.Membership{}, 0, err
	}

	result, err := service.repository.UpdateMemberRole(
		ctx,
		ports.UpdateMemberRoleParams{
			ActorUserID:               input.ActorUserID,
			Membership:                target,
			PreviousRole:              previousRole,
			ExpectedMembershipVersion: input.ExpectedMembershipVersion,
			ExpectedTenantVersion:     current.Tenant.Version,
			NewTenantVersion:          newTenantVersion,
			Event:                     event,
		},
	)
	if err != nil {
		return domain.Membership{}, 0, err
	}

	return result, newTenantVersion, nil
}

// RemoveMember removes a tenant membership.
func (service *Service) RemoveMember(
	ctx context.Context,
	input RemoveMemberInput,
) (int64, error) {
	current, err := service.GetTenantContext(
		ctx,
		input.ActorUserID,
		input.TenantID,
	)
	if err != nil {
		return 0, err
	}

	target, err := service.repository.GetMembership(
		ctx,
		input.TenantID,
		input.UserID,
	)
	if err != nil {
		return 0, err
	}

	if err := requirePermission(
		current,
		domain.PermissionMemberRemove,
	); err != nil {
		return 0, err
	}

	if !domain.CanManageMember(
		current.Membership.Role,
		target.Role,
		target.Role,
	) {
		return 0, domain.ErrForbidden
	}

	if input.ExpectedMembershipVersion != target.Version {
		return 0, domain.ErrVersionConflict
	}

	now := service.now().UTC()
	newTenantVersion := current.Tenant.Version + 1

	event, err := newTenantOutboxEvent(
		ctx,
		service.config.EventTopic,
		"tenant.member_removed",
		input.TenantID,
		newTenantVersion,
		&tenantv1.TenantMemberRemoved{
			TenantId:        input.TenantID,
			UserId:          input.UserID,
			PreviousRole:    protoutil.Role(target.Role),
			RemovedByUserId: input.ActorUserID,
			TenantVersion:   newTenantVersion,
			RemovedAt:       timestamppb.New(now),
		},
		now,
	)
	if err != nil {
		return 0, err
	}

	err = service.repository.RemoveMember(
		ctx,
		ports.RemoveMemberParams{
			ActorUserID:               input.ActorUserID,
			TenantID:                  input.TenantID,
			UserID:                    input.UserID,
			PreviousRole:              target.Role,
			ExpectedMembershipVersion: input.ExpectedMembershipVersion,
			ExpectedTenantVersion:     current.Tenant.Version,
			NewTenantVersion:          newTenantVersion,
			Event:                     event,
		},
	)
	if err != nil {
		return 0, err
	}

	return newTenantVersion, nil
}
