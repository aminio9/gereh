package application

import (
	"context"
	"testing"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testActorID  = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae1"
	testTenantID = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae2"
	testCompany  = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae3"
	testGoalID   = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae4"
	testProject  = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae5"
	testTaskID   = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae6"
)

type mockRepository struct {
	mock.Mock
}

func (repository *mockRepository) CreateGoal(
	ctx context.Context,
	params ports.CreateGoalParams,
) (domain.Goal, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Goal), args.Error(1)
}

func (repository *mockRepository) GetGoal(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	goalID string,
) (domain.Goal, error) {
	args := repository.Called(ctx, actorUserID, tenantID, goalID)
	return args.Get(0).(domain.Goal), args.Error(1)
}

func (repository *mockRepository) ListGoals(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	limit int,
	cursor *ports.GoalCursor,
	includeArchived bool,
) ([]domain.Goal, error) {
	args := repository.Called(ctx, actorUserID, tenantID, companyID, limit, cursor, includeArchived)
	return args.Get(0).([]domain.Goal), args.Error(1)
}

func (repository *mockRepository) UpdateGoal(
	ctx context.Context,
	params ports.UpdateGoalParams,
) (domain.Goal, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Goal), args.Error(1)
}

func (repository *mockRepository) ChangeGoalStatus(
	ctx context.Context,
	params ports.UpdateGoalParams,
) (domain.Goal, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Goal), args.Error(1)
}

func (repository *mockRepository) CreateProject(
	ctx context.Context,
	params ports.CreateProjectParams,
) (domain.Project, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Project), args.Error(1)
}

func (repository *mockRepository) GetProject(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	projectID string,
) (domain.Project, error) {
	args := repository.Called(ctx, actorUserID, tenantID, projectID)
	return args.Get(0).(domain.Project), args.Error(1)
}

func (repository *mockRepository) ListProjects(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	goalID string,
	limit int,
	cursor *ports.ProjectCursor,
	includeArchived bool,
) ([]domain.Project, error) {
	args := repository.Called(ctx, actorUserID, tenantID, companyID, goalID, limit, cursor, includeArchived)
	return args.Get(0).([]domain.Project), args.Error(1)
}

func (repository *mockRepository) UpdateProject(
	ctx context.Context,
	params ports.UpdateProjectParams,
) (domain.Project, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Project), args.Error(1)
}

func (repository *mockRepository) ChangeProjectStatus(
	ctx context.Context,
	params ports.UpdateProjectParams,
) (domain.Project, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Project), args.Error(1)
}

func (repository *mockRepository) CreateTask(
	ctx context.Context,
	params ports.CreateTaskParams,
) (domain.TaskView, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.TaskView), args.Error(1)
}

func (repository *mockRepository) GetTask(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) (domain.TaskView, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID)
	return args.Get(0).(domain.TaskView), args.Error(1)
}

func (repository *mockRepository) ListTasks(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	projectID string,
	limit int,
	cursor *ports.TaskCursor,
	includeCanceled bool,
) ([]domain.TaskView, error) {
	args := repository.Called(ctx, actorUserID, tenantID, companyID, projectID, limit, cursor, includeCanceled)
	return args.Get(0).([]domain.TaskView), args.Error(1)
}

func (repository *mockRepository) UpdateTask(
	ctx context.Context,
	params ports.UpdateTaskParams,
) (domain.TaskView, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.TaskView), args.Error(1)
}

func (repository *mockRepository) ChangeTaskStatus(
	ctx context.Context,
	params ports.TaskChangeParams,
) (domain.TaskView, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.TaskView), args.Error(1)
}

func (repository *mockRepository) AddDependency(
	ctx context.Context,
	params ports.AddDependencyParams,
) (domain.TaskDependency, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.TaskDependency), args.Error(1)
}

func (repository *mockRepository) RemoveDependency(
	ctx context.Context,
	params ports.RemoveDependencyParams,
) error {
	args := repository.Called(ctx, params)
	return args.Error(0)
}

func (repository *mockRepository) AssignTask(
	ctx context.Context,
	params ports.AssignTaskParams,
) (domain.TaskAssignment, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.TaskAssignment), args.Error(1)
}

func (repository *mockRepository) GetAssignment(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	assignmentID string,
) (domain.TaskAssignment, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID, assignmentID)
	return args.Get(0).(domain.TaskAssignment), args.Error(1)
}

func (repository *mockRepository) UnassignTask(
	ctx context.Context,
	params ports.UnassignTaskParams,
) error {
	args := repository.Called(ctx, params)
	return args.Error(0)
}

func (repository *mockRepository) AddComment(
	ctx context.Context,
	params ports.AddCommentParams,
) (domain.Comment, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Comment), args.Error(1)
}

func (repository *mockRepository) GetComment(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	commentID string,
) (domain.Comment, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID, commentID)
	return args.Get(0).(domain.Comment), args.Error(1)
}

func (repository *mockRepository) UpdateComment(
	ctx context.Context,
	params ports.UpdateCommentParams,
) (domain.Comment, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Comment), args.Error(1)
}

func (repository *mockRepository) DeleteComment(
	ctx context.Context,
	params ports.DeleteCommentParams,
) (domain.Comment, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Comment), args.Error(1)
}

func (repository *mockRepository) ListComments(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.Comment, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID)
	return args.Get(0).([]domain.Comment), args.Error(1)
}

func (repository *mockRepository) AddArtifact(
	ctx context.Context,
	params ports.AddArtifactParams,
) (domain.Artifact, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Artifact), args.Error(1)
}

func (repository *mockRepository) GetArtifact(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	artifactID string,
) (domain.Artifact, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID, artifactID)
	return args.Get(0).(domain.Artifact), args.Error(1)
}

func (repository *mockRepository) DeleteArtifact(
	ctx context.Context,
	params ports.DeleteArtifactParams,
) (domain.Artifact, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Artifact), args.Error(1)
}

func (repository *mockRepository) ListArtifacts(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.Artifact, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID)
	return args.Get(0).([]domain.Artifact), args.Error(1)
}

func (repository *mockRepository) AddChecklistItem(
	ctx context.Context,
	params ports.AddChecklistItemParams,
) (domain.ChecklistItem, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.ChecklistItem), args.Error(1)
}

func (repository *mockRepository) GetChecklistItem(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	itemID string,
) (domain.ChecklistItem, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID, itemID)
	return args.Get(0).(domain.ChecklistItem), args.Error(1)
}

func (repository *mockRepository) UpdateChecklistItem(
	ctx context.Context,
	params ports.UpdateChecklistItemParams,
) (domain.ChecklistItem, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.ChecklistItem), args.Error(1)
}

func (repository *mockRepository) DeleteChecklistItem(
	ctx context.Context,
	params ports.DeleteChecklistItemParams,
) error {
	args := repository.Called(ctx, params)
	return args.Error(0)
}

func (repository *mockRepository) ListChecklist(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.ChecklistItem, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID)
	return args.Get(0).([]domain.ChecklistItem), args.Error(1)
}

func (repository *mockRepository) UpsertSchedule(
	ctx context.Context,
	params ports.UpsertScheduleParams,
) (domain.TaskSchedule, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.TaskSchedule), args.Error(1)
}

func (repository *mockRepository) DeleteSchedule(
	ctx context.Context,
	params ports.DeleteScheduleParams,
) error {
	args := repository.Called(ctx, params)
	return args.Error(0)
}

func (repository *mockRepository) GetSchedule(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) (domain.TaskSchedule, error) {
	args := repository.Called(ctx, actorUserID, tenantID, taskID)
	return args.Get(0).(domain.TaskSchedule), args.Error(1)
}

func (repository *mockRepository) ClaimOutbox(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]domain.OutboxRecord, error) {
	args := repository.Called(ctx, limit, lease)
	return args.Get(0).([]domain.OutboxRecord), args.Error(1)
}

func (repository *mockRepository) MarkOutboxPublished(
	ctx context.Context,
	outboxID int64,
) error {
	args := repository.Called(ctx, outboxID)
	return args.Error(0)
}

func (repository *mockRepository) ReleaseOutbox(
	ctx context.Context,
	outboxID int64,
	retryAt time.Time,
	publishError string,
) error {
	args := repository.Called(ctx, outboxID, retryAt, publishError)
	return args.Error(0)
}

type mockAuthorizer struct {
	mock.Mock
}

func (authorizer *mockAuthorizer) Require(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	permission tenantv1.Permission,
) error {
	args := authorizer.Called(ctx, actorUserID, tenantID, permission)
	return args.Error(0)
}

type mockCompanyClient struct {
	mock.Mock
}

func (client *mockCompanyClient) EnsureCompanyActive(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) error {
	args := client.Called(ctx, actorUserID, tenantID, companyID)
	return args.Error(0)
}

func newTestService(
	repository *mockRepository,
	authorizer *mockAuthorizer,
	companyClient *mockCompanyClient,
) *Service {
	service, err := New(
		repository,
		authorizer,
		companyClient,
		Config{
			EventTopic: "gereh.work.events.v1",
		},
	)
	if err != nil {
		panic(err)
	}

	service.now = func() time.Time {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	return service
}

func TestNewRejectsMissingConfig(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)

	_, err := New(
		repository,
		authorizer,
		companyClient,
		Config{},
	)
	require.Error(t, err)

	_, err = New(
		nil,
		authorizer,
		companyClient,
		Config{EventTopic: "a"},
	)
	require.Error(t, err)

	_, err = New(
		repository,
		nil,
		companyClient,
		Config{EventTopic: "a"},
	)
	require.Error(t, err)

	_, err = New(
		repository,
		authorizer,
		nil,
		Config{EventTopic: "a"},
	)
	require.Error(t, err)
}

func TestCreateGoal(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_GOAL_CREATE,
		).
		Return(nil).
		Once()

	companyClient.
		On(
			"EnsureCompanyActive",
			mock.Anything,
			testActorID,
			testTenantID,
			testCompany,
		).
		Return(nil).
		Once()

	repository.
		On(
			"CreateGoal",
			mock.Anything,
			mock.MatchedBy(func(params ports.CreateGoalParams) bool {
				goal := params.Goal
				return goal.TenantID == testTenantID &&
					goal.CompanyID == testCompany &&
					goal.Title == "Ship v1" &&
					goal.Status == domain.GoalStatusActive &&
					goal.Version == 1 &&
					params.Event.Topic == "gereh.work.events.v1"
			}),
		).
		Return(domain.Goal{}, nil).
		Once()

	_, err := service.CreateGoal(
		context.Background(),
		CreateGoalInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompany,
			Title:       "Ship v1",
			Description: "First release",
		},
	)
	require.NoError(t, err)

	authorizer.AssertExpectations(t)
	companyClient.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func TestCreateGoalRejectsForbidden(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_GOAL_CREATE,
		).
		Return(domain.ErrForbidden).
		Once()

	_, err := service.CreateGoal(
		context.Background(),
		CreateGoalInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompany,
			Title:       "Ship v1",
		},
	)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestCreateGoalRejectsInactiveCompany(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_GOAL_CREATE,
		).
		Return(nil).
		Once()

	companyClient.
		On(
			"EnsureCompanyActive",
			mock.Anything,
			testActorID,
			testTenantID,
			testCompany,
		).
		Return(domain.ErrCompanyNotActive).
		Once()

	_, err := service.CreateGoal(
		context.Background(),
		CreateGoalInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompany,
			Title:       "Ship v1",
		},
	)
	require.ErrorIs(t, err, domain.ErrCompanyNotActive)
}

func TestCreateGoalRejectsBlankTitle(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_GOAL_CREATE,
		).
		Return(nil).
		Once()

	companyClient.
		On(
			"EnsureCompanyActive",
			mock.Anything,
			testActorID,
			testTenantID,
			testCompany,
		).
		Return(nil).
		Once()

	_, err := service.CreateGoal(
		context.Background(),
		CreateGoalInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompany,
			Title:       " ",
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestCreateProjectRejectsArchivedGoal(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	now := service.now()

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_PROJECT_CREATE,
		).
		Return(nil).
		Once()

	companyClient.
		On(
			"EnsureCompanyActive",
			mock.Anything,
			testActorID,
			testTenantID,
			testCompany,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetGoal",
			mock.Anything,
			testActorID,
			testTenantID,
			testGoalID,
		).
		Return(domain.Goal{
			TenantID:  testTenantID,
			CompanyID: testCompany,
			ID:        testGoalID,
			Title:     "Archived goal",
			Status:    domain.GoalStatusArchived,
			Version:   2,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil).
		Once()

	_, err := service.CreateProject(
		context.Background(),
		CreateProjectInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompany,
			GoalID:      testGoalID,
			Title:       "Release project",
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestCreateTaskRejectsArchivedProject(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	now := service.now()

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_TASK_CREATE,
		).
		Return(nil).
		Once()

	companyClient.
		On(
			"EnsureCompanyActive",
			mock.Anything,
			testActorID,
			testTenantID,
			testCompany,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetProject",
			mock.Anything,
			testActorID,
			testTenantID,
			testProject,
		).
		Return(domain.Project{
			TenantID:  testTenantID,
			CompanyID: testCompany,
			GoalID:    testGoalID,
			ID:        testProject,
			Title:     "Archived project",
			Status:    domain.ProjectStatusArchived,
			Version:   2,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil).
		Once()

	_, err := service.CreateTask(
		context.Background(),
		CreateTaskInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompany,
			ProjectID:   testProject,
			Title:       "Do the work",
			Priority:    workv1.TaskPriority_TASK_PRIORITY_NORMAL,
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestChangeGoalStatusRejectsInvalidTransition(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	now := service.now()

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_GOAL_UPDATE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetGoal",
			mock.Anything,
			testActorID,
			testTenantID,
			testGoalID,
		).
		Return(domain.Goal{
			TenantID:  testTenantID,
			CompanyID: testCompany,
			ID:        testGoalID,
			Title:     "Goal",
			Status:    domain.GoalStatusActive,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil).
		Once()

	_, err := service.ChangeGoalStatus(
		context.Background(),
		testActorID,
		testTenantID,
		testGoalID,
		1,
		workv1.GoalStatus_GOAL_STATUS_UNSPECIFIED,
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestChangeGoalStatusCommits(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	now := service.now()

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_GOAL_UPDATE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetGoal",
			mock.Anything,
			testActorID,
			testTenantID,
			testGoalID,
		).
		Return(domain.Goal{
			TenantID:  testTenantID,
			CompanyID: testCompany,
			ID:        testGoalID,
			Title:     "Goal",
			Status:    domain.GoalStatusActive,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil).
		Once()

	repository.
		On(
			"ChangeGoalStatus",
			mock.Anything,
			mock.MatchedBy(func(params ports.UpdateGoalParams) bool {
				return params.Goal.ID == testGoalID &&
					params.Goal.Status == domain.GoalStatusArchived &&
					params.ExpectedVersion == 1
			}),
		).
		Return(domain.Goal{}, nil).
		Once()

	_, err := service.ChangeGoalStatus(
		context.Background(),
		testActorID,
		testTenantID,
		testGoalID,
		1,
		workv1.GoalStatus_GOAL_STATUS_ARCHIVED,
	)
	require.NoError(t, err)

	repository.AssertExpectations(t)
}

func TestChangeTaskStatusRejectsInvalidTransition(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	now := service.now()

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_TASK_STATUS_UPDATE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetTask",
			mock.Anything,
			testActorID,
			testTenantID,
			testTaskID,
		).
		Return(domain.TaskView{
			Task: domain.Task{
				TenantID:  testTenantID,
				CompanyID: testCompany,
				ProjectID: testProject,
				ID:        testTaskID,
				Title:     "Task",
				Status:    domain.TaskStatusCompleted,
				Priority:  domain.TaskPriorityNormal,
				Version:   1,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}, nil).
		Once()

	_, err := service.ChangeTaskStatus(
		context.Background(),
		testActorID,
		testTenantID,
		testTaskID,
		1,
		workv1.TaskStatus_TASK_STATUS_IN_PROGRESS,
	)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestChangeTaskStatusCommits(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	now := service.now()

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_TASK_STATUS_UPDATE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetTask",
			mock.Anything,
			testActorID,
			testTenantID,
			testTaskID,
		).
		Return(domain.TaskView{
			Task: domain.Task{
				TenantID:  testTenantID,
				CompanyID: testCompany,
				ProjectID: testProject,
				ID:        testTaskID,
				Title:     "Task",
				Status:    domain.TaskStatusInProgress,
				Priority:  domain.TaskPriorityNormal,
				Version:   2,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}, nil).
		Once()

	repository.
		On(
			"ChangeTaskStatus",
			mock.Anything,
			mock.MatchedBy(func(params ports.TaskChangeParams) bool {
				return params.Task.ID == testTaskID &&
					params.Task.Status == domain.TaskStatusCompleted &&
					params.PreviousStatus == domain.TaskStatusInProgress &&
					params.ExpectedVersion == 2
			}),
		).
		Return(domain.TaskView{}, nil).
		Once()

	_, err := service.ChangeTaskStatus(
		context.Background(),
		testActorID,
		testTenantID,
		testTaskID,
		2,
		workv1.TaskStatus_TASK_STATUS_COMPLETED,
	)
	require.NoError(t, err)

	repository.AssertExpectations(t)
}

func TestAddCommentRejectsMismatchedAuthor(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_TASK_COMMENT_CREATE,
		).
		Return(nil).
		Once()

	otherUser := "018f7767-28d2-7f5c-a693-0bb4c8ee4ae7"

	_, err := service.AddComment(
		context.Background(),
		AddCommentInput{
			ActorUserID:  testActorID,
			TenantID:     testTenantID,
			TaskID:       testTaskID,
			AuthorType:   workv1.CommentAuthorType_COMMENT_AUTHOR_TYPE_USER,
			AuthorUserID: &otherUser,
			Body:         "Hello",
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestUpdateCommentRejectsNonOwnerWithoutModerate(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	now := service.now()

	otherUser := "018f7767-28d2-7f5c-a693-0bb4c8ee4ae7"
	commentID := "018f7767-28d2-7f5c-a693-0bb4c8ee4ae8"

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_TASK_COMMENT_MODERATE,
		).
		Return(domain.ErrForbidden).
		Once()

	repository.
		On(
			"GetComment",
			mock.Anything,
			testActorID,
			testTenantID,
			testTaskID,
			commentID,
		).
		Return(domain.Comment{
			TenantID:     testTenantID,
			TaskID:       testTaskID,
			ID:           commentID,
			AuthorType:   domain.AssigneeTypeUser,
			AuthorUserID: &otherUser,
			Body:         "Original",
			Version:      1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}, nil).
		Once()

	_, err := service.UpdateComment(
		context.Background(),
		UpdateCommentInput{
			ActorUserID:     testActorID,
			TenantID:        testTenantID,
			TaskID:          testTaskID,
			CommentID:       commentID,
			ExpectedVersion: 1,
			Body:            "Edited",
		},
	)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestUpdateCommentAllowsModerator(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	now := service.now()

	otherUser := "018f7767-28d2-7f5c-a693-0bb4c8ee4ae7"
	commentID := "018f7767-28d2-7f5c-a693-0bb4c8ee4ae8"

	repository.
		On(
			"GetComment",
			mock.Anything,
			testActorID,
			testTenantID,
			testTaskID,
			commentID,
		).
		Return(domain.Comment{
			TenantID:     testTenantID,
			TaskID:       testTaskID,
			ID:           commentID,
			AuthorType:   domain.AssigneeTypeUser,
			AuthorUserID: &otherUser,
			Body:         "Original",
			Version:      1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}, nil).
		Once()

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_TASK_COMMENT_MODERATE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"UpdateComment",
			mock.Anything,
			mock.MatchedBy(func(params ports.UpdateCommentParams) bool {
				return params.Comment.ID == commentID &&
					params.Comment.Body == "Edited" &&
					params.Comment.Version == 2
			}),
		).
		Return(domain.Comment{}, nil).
		Once()

	_, err := service.UpdateComment(
		context.Background(),
		UpdateCommentInput{
			ActorUserID:     testActorID,
			TenantID:        testTenantID,
			TaskID:          testTaskID,
			CommentID:       commentID,
			ExpectedVersion: 1,
			Body:            "Edited",
		},
	)
	require.NoError(t, err)

	repository.AssertExpectations(t)
}

func TestListGoalsNextToken(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_WORK_READ,
		).
		Return(nil).
		Once()

	now := service.now()

	var fullPage []domain.Goal

	for index := 0; index < 50; index++ {
		fullPage = append(fullPage, domain.Goal{
			TenantID:  testTenantID,
			CompanyID: testCompany,
			ID:        testGoalID,
			Title:     "Goal",
			Status:    domain.GoalStatusActive,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	repository.
		On(
			"ListGoals",
			mock.Anything,
			testActorID,
			testTenantID,
			testCompany,
			50,
			(*ports.GoalCursor)(nil),
			false,
		).
		Return(
			fullPage,
			nil,
		).
		Once()

	goals, nextToken, err := service.ListGoals(
		context.Background(),
		testActorID,
		testTenantID,
		testCompany,
		50,
		"",
		false,
	)
	require.NoError(t, err)
	require.Len(t, goals, 50)
	require.NotEmpty(t, nextToken)
}

func TestAddArtifactRejectsBadSHA256(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_TASK_ARTIFACT_MANAGE,
		).
		Return(nil).
		Once()

	_, err := service.AddArtifact(
		context.Background(),
		AddArtifactInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompany,
			TaskID:      testTaskID,
			ObjectKey:   "uploads/object",
			FileName:    "report.pdf",
			ContentType: "application/pdf",
			SizeBytes:   100,
			SHA256:      "not-a-sha256",
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestUpsertScheduleRejectsInvertedWindow(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	companyClient := new(mockCompanyClient)
	service := newTestService(repository, authorizer, companyClient)

	notBefore := service.now()
	dueAt := notBefore.Add(-time.Hour)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_TASK_SCHEDULE_MANAGE,
		).
		Return(nil).
		Once()

	_, err := service.UpsertTaskSchedule(
		context.Background(),
		UpsertScheduleInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			TaskID:      testTaskID,
			NotBefore:   &notBefore,
			DueAt:       &dueAt,
			Timezone:    "UTC",
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}
