package temporalapp

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
)

type Starter struct {
	client    client.Client
	taskQueue string
}

func NewStarter(temporalClient client.Client, taskQueue string) *Starter {
	return &Starter{client: temporalClient, taskQueue: taskQueue}
}

func (s *Starter) Start(ctx context.Context, downloadID int64, deadline time.Time) error {
	_, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        fmt.Sprintf("download-%d", downloadID),
		TaskQueue: s.taskQueue,
	}, WorkflowName, WorkflowInput{
		DownloadID: downloadID,
		Deadline:   deadline,
	})
	if err != nil {
		return fmt.Errorf("execute workflow: %w", err)
	}
	return nil
}
