package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func workIntegrationTest(t *testing.T) *Repository {
	t.Helper()

	databaseURL := os.Getenv("WORK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("WORK_TEST_DATABASE_URL is not configured")
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

func mustUUID(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate UUIDv7: %v", err)
	}

	return id.String()
}

func testEvent(
	t *testing.T,
	_ string,
) domain.OutboxEvent {
	t.Helper()

	return domain.OutboxEvent{
		ID:         mustUUID(t),
		Topic:      "gereh.work.events.v1",
		Key:        mustUUID(t),
		Envelope:   []byte("{}"),
		OccurredAt: time.Now().UTC(),
	}
}

func cleanupTenant(
	t *testing.T,
	repository *Repository,
	tenantID string,
) {
	t.Helper()

	ctx := context.Background()

	if _, err := repository.pool.Exec(
		ctx,
		`
			DELETE FROM work_outbox
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	); err != nil {
		t.Errorf("cleanup work outbox: %v", err)
	}

	if _, err := repository.pool.Exec(
		ctx,
		`
			DELETE FROM work_goals
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	); err != nil {
		t.Errorf("cleanup work goals: %v", err)
	}
}

func createTestGoal(
	ctx context.Context,
	t *testing.T,
	repository *Repository,
	tenantID string,
	companyID string,
	actorUserID string,
	title string,
) domain.Goal {
	t.Helper()

	goal, err := repository.CreateGoal(
		ctx,
		ports.CreateGoalParams{
			ActorUserID: actorUserID,
			Goal: domain.Goal{
				TenantID:        tenantID,
				CompanyID:       companyID,
				ID:              mustUUID(t),
				Title:           title,
				Description:     "Integration test goal",
				Status:          domain.GoalStatusActive,
				Version:         1,
				CreatedByUserID: actorUserID,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			},
			Event: testEvent(t, tenantID),
		},
	)
	require.NoError(t, err)

	return goal
}

func createTestProject(
	ctx context.Context,
	t *testing.T,
	repository *Repository,
	tenantID string,
	companyID string,
	goalID string,
	actorUserID string,
	title string,
) domain.Project {
	t.Helper()

	project, err := repository.CreateProject(
		ctx,
		ports.CreateProjectParams{
			ActorUserID: actorUserID,
			Project: domain.Project{
				TenantID:        tenantID,
				CompanyID:       companyID,
				GoalID:          goalID,
				ID:              mustUUID(t),
				Title:           title,
				Description:     "Integration test project",
				Status:          domain.ProjectStatusPlanned,
				Version:         1,
				CreatedByUserID: actorUserID,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			},
			Event: testEvent(t, tenantID),
		},
	)
	require.NoError(t, err)

	return project
}

func createTestTask(
	ctx context.Context,
	t *testing.T,
	repository *Repository,
	tenantID string,
	companyID string,
	projectID string,
	actorUserID string,
	title string,
) domain.Task {
	t.Helper()

	task, err := repository.CreateTask(
		ctx,
		ports.CreateTaskParams{
			ActorUserID: actorUserID,
			Task: domain.Task{
				TenantID:        tenantID,
				CompanyID:       companyID,
				ProjectID:       projectID,
				ID:              mustUUID(t),
				Title:           title,
				Description:     "Integration test task",
				Status:          domain.TaskStatusBacklog,
				Priority:        domain.TaskPriorityNormal,
				Version:         1,
				CreatedByUserID: actorUserID,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			},
			Event: testEvent(t, tenantID),
		},
	)
	require.NoError(t, err)

	return task.Task
}

func TestGoalLifecycle(t *testing.T) {
	repository := workIntegrationTest(t)

	ctx := context.Background()

	actorUserID := mustUUID(t)
	tenantID := mustUUID(t)
	companyID := mustUUID(t)

	t.Cleanup(func() {
		cleanupTenant(t, repository, tenantID)
	})

	goal := createTestGoal(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		actorUserID,
		"Ship v1",
	)

	loaded, err := repository.GetGoal(
		ctx,
		actorUserID,
		tenantID,
		goal.ID,
	)
	require.NoError(t, err)
	require.Equal(t, goal.ID, loaded.ID)
	require.Equal(t, domain.GoalStatusActive, loaded.Status)
	require.Equal(t, int64(1), loaded.Version)

	goals, err := repository.ListGoals(
		ctx,
		actorUserID,
		tenantID,
		companyID,
		50,
		nil,
		false,
	)
	require.NoError(t, err)
	require.Len(t, goals, 1)

	loaded.Title = "Ship v1.1"
	loaded.Description = "Updated"
	loaded.Version = 2

	updated, err := repository.UpdateGoal(
		ctx,
		ports.UpdateGoalParams{
			ActorUserID:     actorUserID,
			Goal:            loaded,
			ExpectedVersion: 1,
			Event:           testEvent(t, tenantID),
		},
	)
	require.NoError(t, err)
	require.Equal(t, "Ship v1.1", updated.Title)

	now := time.Now().UTC()

	loaded.Version = 3
	loaded.Status = domain.GoalStatusCompleted
	loaded.CompletedAt = &now

	changed, err := repository.ChangeGoalStatus(
		ctx,
		ports.UpdateGoalParams{
			ActorUserID:     actorUserID,
			Goal:            loaded,
			ExpectedVersion: 2,
			Event:           testEvent(t, tenantID),
		},
	)
	require.NoError(t, err)
	require.Equal(t, domain.GoalStatusCompleted, changed.Status)
}

func TestGoalUpdateRejectsStaleVersion(t *testing.T) {
	repository := workIntegrationTest(t)

	ctx := context.Background()

	actorUserID := mustUUID(t)
	tenantID := mustUUID(t)
	companyID := mustUUID(t)

	t.Cleanup(func() {
		cleanupTenant(t, repository, tenantID)
	})

	goal := createTestGoal(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		actorUserID,
		"Ship v1",
	)

	goal.Title = "Concurrent edit"

	_, err := repository.UpdateGoal(
		ctx,
		ports.UpdateGoalParams{
			ActorUserID:     actorUserID,
			Goal:            goal,
			ExpectedVersion: 5,
			Event:           testEvent(t, tenantID),
		},
	)
	require.ErrorIs(t, err, domain.ErrVersionConflict)
}

func TestRLSIsolatesTenants(t *testing.T) {
	repository := workIntegrationTest(t)

	ctx := context.Background()

	actorUserID := mustUUID(t)
	tenantA := mustUUID(t)
	tenantB := mustUUID(t)
	companyA := mustUUID(t)
	companyB := mustUUID(t)

	t.Cleanup(func() {
		cleanupTenant(t, repository, tenantA)
		cleanupTenant(t, repository, tenantB)
	})

	goal := createTestGoal(
		ctx,
		t,
		repository,
		tenantA,
		companyA,
		actorUserID,
		"Tenant A goal",
	)

	_, err := repository.GetGoal(
		ctx,
		actorUserID,
		tenantB,
		goal.ID,
	)
	require.ErrorIs(t, err, domain.ErrNotFound)

	goals, err := repository.ListGoals(
		ctx,
		actorUserID,
		tenantB,
		companyB,
		50,
		nil,
		false,
	)
	require.NoError(t, err)
	require.Empty(t, goals)
}

func TestGoalStatusChangeRejectsOpenProjects(t *testing.T) {
	repository := workIntegrationTest(t)

	ctx := context.Background()

	actorUserID := mustUUID(t)
	tenantID := mustUUID(t)
	companyID := mustUUID(t)

	t.Cleanup(func() {
		cleanupTenant(t, repository, tenantID)
	})

	goal := createTestGoal(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		actorUserID,
		"Ship v1",
	)

	createTestProject(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		goal.ID,
		actorUserID,
		"Release",
	)

	now := time.Now().UTC()

	goal.Version = 2
	goal.Status = domain.GoalStatusCompleted
	goal.CompletedAt = &now

	_, err := repository.ChangeGoalStatus(
		ctx,
		ports.UpdateGoalParams{
			ActorUserID:     actorUserID,
			Goal:            goal,
			ExpectedVersion: 1,
			Event:           testEvent(t, tenantID),
		},
	)
	require.ErrorIs(t, err, domain.ErrGoalOpenProjects)
}

func TestTaskStatusChangeRejectsIncompleteDependencies(t *testing.T) {
	repository := workIntegrationTest(t)

	ctx := context.Background()

	actorUserID := mustUUID(t)
	tenantID := mustUUID(t)
	companyID := mustUUID(t)

	t.Cleanup(func() {
		cleanupTenant(t, repository, tenantID)
	})

	goal := createTestGoal(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		actorUserID,
		"Ship v1",
	)

	project := createTestProject(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		goal.ID,
		actorUserID,
		"Release",
	)

	prerequisite := createTestTask(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		project.ID,
		actorUserID,
		"Prerequisite",
	)

	dependent := createTestTask(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		project.ID,
		actorUserID,
		"Dependent",
	)

	_, err := repository.AddDependency(
		ctx,
		ports.AddDependencyParams{
			ActorUserID: actorUserID,
			Dependency: domain.TaskDependency{
				TenantID:        tenantID,
				TaskID:          dependent.ID,
				DependsOnTaskID: prerequisite.ID,
				CreatedByUserID: actorUserID,
				CreatedAt:       time.Now().UTC(),
			},
			Event: testEvent(t, tenantID),
		},
	)
	require.NoError(t, err)

	now := time.Now().UTC()

	dependent.Status = domain.TaskStatusCompleted
	dependent.Version = 2
	dependent.CompletedAt = &now

	_, err = repository.ChangeTaskStatus(
		ctx,
		ports.TaskChangeParams{
			ActorUserID:     actorUserID,
			Task:            dependent,
			PreviousStatus:  domain.TaskStatusBacklog,
			ExpectedVersion: 1,
			Event:           testEvent(t, tenantID),
		},
	)
	require.ErrorIs(t, err, domain.ErrTaskBlocked)
}

// TestDependencyCycleIsRejected inserts the two edges of a cycle
// concurrently. The project-level lock serializes the mutations and the
// recursive cycle check must reject exactly one of them.
func TestDependencyCycleIsRejected(t *testing.T) {
	repository := workIntegrationTest(t)

	ctx := context.Background()

	actorUserID := mustUUID(t)
	tenantID := mustUUID(t)
	companyID := mustUUID(t)

	t.Cleanup(func() {
		cleanupTenant(t, repository, tenantID)
	})

	goal := createTestGoal(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		actorUserID,
		"Ship v1",
	)

	project := createTestProject(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		goal.ID,
		actorUserID,
		"Release",
	)

	taskA := createTestTask(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		project.ID,
		actorUserID,
		"Task A",
	)

	taskB := createTestTask(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		project.ID,
		actorUserID,
		"Task B",
	)

	var waitGroup sync.WaitGroup

	errorsChannel := make(chan error, 2)

	waitGroup.Add(2)

	go func() {
		defer waitGroup.Done()

		_, err := repository.AddDependency(
			ctx,
			ports.AddDependencyParams{
				ActorUserID: actorUserID,
				Dependency: domain.TaskDependency{
					TenantID:        tenantID,
					TaskID:          taskA.ID,
					DependsOnTaskID: taskB.ID,
					CreatedByUserID: actorUserID,
					CreatedAt:       time.Now().UTC(),
				},
				Event: testEvent(t, tenantID),
			},
		)

		errorsChannel <- err
	}()

	go func() {
		defer waitGroup.Done()

		_, err := repository.AddDependency(
			ctx,
			ports.AddDependencyParams{
				ActorUserID: actorUserID,
				Dependency: domain.TaskDependency{
					TenantID:        tenantID,
					TaskID:          taskB.ID,
					DependsOnTaskID: taskA.ID,
					CreatedByUserID: actorUserID,
					CreatedAt:       time.Now().UTC(),
				},
				Event: testEvent(t, tenantID),
			},
		)

		errorsChannel <- err
	}()

	waitGroup.Wait()

	close(errorsChannel)

	var firstError error

	for err := range errorsChannel {
		firstError = err

		if !errors.Is(err, domain.ErrDependencyCycle) {
			require.NoError(t, err)
		}
	}

	require.ErrorIs(t, firstError, domain.ErrDependencyCycle)
}

func TestCanceledPrerequisiteRemainsBlocking(
	t *testing.T,
) {
	repository := workIntegrationTest(t)

	ctx := context.Background()

	actorUserID := mustUUID(t)
	tenantID := mustUUID(t)
	companyID := mustUUID(t)

	t.Cleanup(func() {
		cleanupTenant(t, repository, tenantID)
	})

	goal := createTestGoal(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		actorUserID,
		"Ship v1",
	)

	project := createTestProject(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		goal.ID,
		actorUserID,
		"Release",
	)

	prerequisite := createTestTask(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		project.ID,
		actorUserID,
		"Prerequisite",
	)

	dependent := createTestTask(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		project.ID,
		actorUserID,
		"Dependent",
	)

	_, err := repository.AddDependency(
		ctx,
		ports.AddDependencyParams{
			ActorUserID: actorUserID,
			Dependency: domain.TaskDependency{
				TenantID:        tenantID,
				TaskID:          dependent.ID,
				DependsOnTaskID: prerequisite.ID,
				CreatedByUserID: actorUserID,
				CreatedAt:       time.Now().UTC(),
			},
			Event: testEvent(t, tenantID),
		},
	)
	require.NoError(t, err)

	now := time.Now().UTC()

	prerequisite.Status = domain.TaskStatusCanceled
	prerequisite.Version = 2
	prerequisite.CanceledAt = &now

	_, err = repository.ChangeTaskStatus(
		ctx,
		ports.TaskChangeParams{
			ActorUserID:     actorUserID,
			Task:            prerequisite,
			PreviousStatus:  domain.TaskStatusBacklog,
			ExpectedVersion: 1,
			Event:           testEvent(t, tenantID),
		},
	)
	require.NoError(t, err)

	current, err := repository.GetTask(
		ctx,
		actorUserID,
		tenantID,
		dependent.ID,
	)
	require.NoError(t, err)

	require.True(t, current.Blocked)
	require.EqualValues(
		t,
		1,
		current.IncompleteDependencyCount,
	)
}

func TestOutboxClaimPublishRelease(t *testing.T) {
	repository := workIntegrationTest(t)

	ctx := context.Background()

	actorUserID := mustUUID(t)
	tenantID := mustUUID(t)
	companyID := mustUUID(t)

	t.Cleanup(func() {
		cleanupTenant(t, repository, tenantID)
	})

	createTestGoal(
		ctx,
		t,
		repository,
		tenantID,
		companyID,
		actorUserID,
		"Ship v1",
	)

	records, err := repository.ClaimOutbox(
		ctx,
		100,
		30*time.Second,
	)
	require.NoError(t, err)
	require.NotEmpty(t, records)

	for _, record := range records {
		err := repository.MarkOutboxPublished(
			ctx,
			record.OutboxID,
		)
		require.NoError(t, err)
	}

	records, err = repository.ClaimOutbox(
		ctx,
		100,
		30*time.Second,
	)
	require.NoError(t, err)
	require.Empty(t, records)
}
