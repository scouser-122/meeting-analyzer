package service

import (
	"context"

	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
)

// TasksService service to work with tasks
type TasksService struct {
	tasksRepo       repository.TaskRepository
	repositoryUtils repository.RepositoryUtils
}

// NewTasksService creates new TasksService instance.
func NewTasksService(
	tasksRepo repository.TaskRepository,
	repositoryUtils repository.RepositoryUtils,
) *TasksService {
	service := TasksService{}
	service.tasksRepo = tasksRepo
	service.repositoryUtils = repositoryUtils
	return &service
}

// CreateNewTask creates a new processing task for the specified meeting.
func (t *TasksService) CreateNewTask(ctx context.Context, meetingID string) (*model.Task, error) {
	task, err := t.tasksRepo.Create(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// UpdateStatus updates the task status and optional error message.
func (t *TasksService) UpdateStatus(ctx context.Context, taskID string, status model.TaskStatus, errorMessage *string) error {
	return t.tasksRepo.UpdateStatus(ctx, taskID, status, errorMessage)
}

// GetStatus returns the current status of the task for the specified meeting.
func (t *TasksService) GetStatus(ctx context.Context, meetingID string) (model.TaskStatus, error) {
	task, err := t.tasksRepo.GetByMeetingID(ctx, meetingID)
	if err != nil {
		return "", err
	}
	return task.Status, nil
}

// GetByMeetingID returns the task associated with the specified meeting.
func (t *TasksService) GetByMeetingID(ctx context.Context, meetingID string) (*model.Task, error) {
	task, err := t.tasksRepo.GetByMeetingID(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	return task, nil
}
