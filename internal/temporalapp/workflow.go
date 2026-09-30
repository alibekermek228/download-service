package temporalapp

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func DownloadWorkflow(ctx workflow.Context, input WorkflowInput) error {
	remaining := input.Deadline.Sub(workflow.Now(ctx))
	if remaining < time.Second {
		remaining = time.Second
	}

	downloadContext := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: remaining + 30*time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 3,
		},
	})
	if err := workflow.ExecuteActivity(
		downloadContext,
		ActivityDownloadBatch,
		input,
	).Get(downloadContext, nil); err != nil {
		cleanupContext := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 3,
			},
		})
		_ = workflow.ExecuteActivity(
			cleanupContext,
			ActivityFailDownload,
			input.DownloadID,
		).Get(cleanupContext, nil)
		return err
	}

	finishContext := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})
	return workflow.ExecuteActivity(
		finishContext,
		ActivityMarkDone,
		input.DownloadID,
	).Get(finishContext, nil)
}
