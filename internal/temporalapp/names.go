package temporalapp

import "time"

const (
	WorkflowName            = "DownloadWorkflow"
	ActivityDownloadBatch   = "DownloadActivities.DownloadBatch"
	ActivitySaveFileResults = "DownloadActivities.SaveFileResults"
	ActivityMarkDone        = "DownloadActivities.MarkDone"
	ActivityFailDownload    = "DownloadActivities.FailDownload"
)

type WorkflowInput struct {
	DownloadID int64
	Deadline   time.Time
}
