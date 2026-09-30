package domain

import "time"

type DownloadStatus string

const (
	StatusProcess DownloadStatus = "PROCESS"
	StatusDone    DownloadStatus = "DONE"
)

type Download struct {
	ID        int64
	Status    DownloadStatus
	Timeout   time.Duration
	CreatedAt time.Time
	Files     []File
}

type File struct {
	ID         int64
	DownloadID int64
	Position   int
	URL        string
	Data       []byte
	ErrorCode  string
}

const (
	ErrorTimeout      = "TIMEOUT"
	ErrorInvalidURL   = "INVALID_URL"
	ErrorHTTPStatus   = "HTTP_STATUS"
	ErrorFileTooLarge = "FILE_TOO_LARGE"
	ErrorDownload     = "DOWNLOAD_ERROR"
	ErrorWorkflow     = "WORKFLOW_START_ERROR"
)
