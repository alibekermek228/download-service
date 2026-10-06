package temporalapp

import (
	"context"
	"fmt"
	"sync"

	"internship-download-service/internal/domain"
	"internship-download-service/internal/downloader"
)

type ActivityRepository interface {
	ListPendingFiles(ctx context.Context, downloadID int64) ([]domain.File, error)
	SaveFileResult(ctx context.Context, downloadID, fileID int64, data []byte, errorCode string) error
	MarkDone(ctx context.Context, downloadID int64) error
	FailDownload(ctx context.Context, downloadID int64, errorCode string) error
}

type FileDownloader interface {
	Download(ctx context.Context, rawURL string) ([]byte, error)
}

type Activities struct {
	repository    ActivityRepository
	downloader    FileDownloader
	maxConcurrent int
}

func NewActivities(
	repository ActivityRepository,
	fileDownloader FileDownloader,
	maxConcurrent int,
) *Activities {
	return &Activities{
		repository:    repository,
		downloader:    fileDownloader,
		maxConcurrent: maxConcurrent,
	}
}

type FileResult struct {
	FileID    int64
	Data      []byte
	ErrorCode string
}

type SaveResultsInput struct {
	DownloadID int64
	Results    []FileResult
}

func (a *Activities) DownloadBatch(ctx context.Context, input WorkflowInput) ([]FileResult, error) {
	files, err := a.repository.ListPendingFiles(ctx, input.DownloadID)
	if err != nil {
		return nil, fmt.Errorf("list pending files: %w", err)
	}

	downloadContext, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()

	semaphore := make(chan struct{}, a.maxConcurrent)
	results := make([]FileResult, 0, len(files))
	var waitGroup sync.WaitGroup
	var mutex sync.Mutex

	for _, file := range files {
		file := file
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			result := FileResult{FileID: file.ID}
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-downloadContext.Done():
				result.ErrorCode = domain.ErrorTimeout
				mutex.Lock()
				results = append(results, result)
				mutex.Unlock()
				return
			}

			data, downloadErr := a.downloader.Download(downloadContext, file.URL)
			if downloadErr != nil {
				result.ErrorCode = downloader.ErrorCode(downloadErr)
			} else {
				result.Data = data
			}

			mutex.Lock()
			results = append(results, result)
			mutex.Unlock()
		}()
	}

	waitGroup.Wait()

	return results, nil
}

func (a *Activities) SaveFileResults(ctx context.Context, input SaveResultsInput) error {
	for _, result := range input.Results {
		if err := a.repository.SaveFileResult(
			ctx,
			input.DownloadID,
			result.FileID,
			result.Data,
			result.ErrorCode,
		); err != nil {
			return fmt.Errorf("save file %d result: %w", result.FileID, err)
		}
	}

	return nil
}

func (a *Activities) MarkDone(ctx context.Context, downloadID int64) error {
	if err := a.repository.MarkDone(ctx, downloadID); err != nil {
		return fmt.Errorf("mark done: %w", err)
	}
	return nil
}

func (a *Activities) FailDownload(ctx context.Context, downloadID int64) error {
	if err := a.repository.FailDownload(ctx, downloadID, domain.ErrorDownload); err != nil {
		return fmt.Errorf("fail download: %w", err)
	}
	return nil
}
