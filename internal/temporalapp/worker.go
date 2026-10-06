package temporalapp

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func NewWorker(
	temporalClient client.Client,
	taskQueue string,
	activities *Activities,
) worker.Worker {
	temporalWorker := worker.New(temporalClient, taskQueue, worker.Options{})
	temporalWorker.RegisterWorkflowWithOptions(
		DownloadWorkflow,
		workflow.RegisterOptions{Name: WorkflowName},
	)
	temporalWorker.RegisterActivityWithOptions(
		activities.DownloadBatch,
		activity.RegisterOptions{Name: ActivityDownloadBatch},
	)
	temporalWorker.RegisterActivityWithOptions(
		activities.SaveFileResults,
		activity.RegisterOptions{Name: ActivitySaveFileResults},
	)
	temporalWorker.RegisterActivityWithOptions(
		activities.MarkDone,
		activity.RegisterOptions{Name: ActivityMarkDone},
	)
	temporalWorker.RegisterActivityWithOptions(
		activities.FailDownload,
		activity.RegisterOptions{Name: ActivityFailDownload},
	)
	return temporalWorker
}
