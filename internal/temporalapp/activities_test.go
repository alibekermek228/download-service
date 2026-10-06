package temporalapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"internship-download-service/internal/domain"
)

type activityRepositoryStub struct {
	files   []domain.File
	results map[int64]FileResult
}

func (r *activityRepositoryStub) ListPendingFiles(context.Context, int64) ([]domain.File, error) {
	return r.files, nil
}

func (r *activityRepositoryStub) SaveFileResult(
	_ context.Context,
	_ int64,
	fileID int64,
	data []byte,
	errorCode string,
) error {
	if r.results == nil {
		r.results = make(map[int64]FileResult)
	}
	r.results[fileID] = FileResult{FileID: fileID, Data: data, ErrorCode: errorCode}
	return nil
}

func (r *activityRepositoryStub) MarkDone(context.Context, int64) error { return nil }

func (r *activityRepositoryStub) FailDownload(context.Context, int64, string) error { return nil }

type activityDownloaderStub struct {
	download func(context.Context, string) ([]byte, error)
}

func (d activityDownloaderStub) Download(ctx context.Context, rawURL string) ([]byte, error) {
	return d.download(ctx, rawURL)
}

func TestDownloadBatchContinuesAfterFileError(t *testing.T) {
	t.Parallel()

	repository := &activityRepositoryStub{files: []domain.File{
		{ID: 1, URL: "https://example.com/ok"},
		{ID: 2, URL: "https://example.com/error"},
	}}
	fileDownloader := activityDownloaderStub{download: func(_ context.Context, rawURL string) ([]byte, error) {
		if rawURL == "https://example.com/error" {
			return nil, errors.New("network error")
		}
		return []byte("ok"), nil
	}}
	activities := NewActivities(repository, fileDownloader, 2)

	results, err := activities.DownloadBatch(context.Background(), WorkflowInput{
		DownloadID: 10,
		Deadline:   time.Now().Add(time.Second),
	})
	if err != nil {
		t.Fatalf("DownloadBatch() error = %v", err)
	}
	resultByFileID := resultsByFileID(results)
	if string(resultByFileID[1].Data) != "ok" {
		t.Fatalf("successful file data = %q, want ok", resultByFileID[1].Data)
	}
	if resultByFileID[2].ErrorCode != domain.ErrorDownload {
		t.Fatalf("failed file code = %q, want %q", resultByFileID[2].ErrorCode, domain.ErrorDownload)
	}
}

func TestSaveFileResults(t *testing.T) {
	t.Parallel()

	repository := &activityRepositoryStub{}
	activities := NewActivities(repository, nil, 1)

	err := activities.SaveFileResults(context.Background(), SaveResultsInput{
		DownloadID: 12,
		Results: []FileResult{
			{FileID: 1, Data: []byte("ok")},
			{FileID: 2, ErrorCode: domain.ErrorDownload},
		},
	})
	if err != nil {
		t.Fatalf("SaveFileResults() error = %v", err)
	}
	if string(repository.results[1].Data) != "ok" {
		t.Fatalf("saved file data = %q, want ok", repository.results[1].Data)
	}
	if repository.results[2].ErrorCode != domain.ErrorDownload {
		t.Fatalf("saved file code = %q, want %q", repository.results[2].ErrorCode, domain.ErrorDownload)
	}
}

func TestDownloadBatchStopsOnDeadline(t *testing.T) {
	t.Parallel()

	repository := &activityRepositoryStub{files: []domain.File{
		{ID: 1, URL: "https://example.com/slow"},
	}}
	fileDownloader := activityDownloaderStub{download: func(ctx context.Context, _ string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	activities := NewActivities(repository, fileDownloader, 1)

	results, err := activities.DownloadBatch(context.Background(), WorkflowInput{
		DownloadID: 11,
		Deadline:   time.Now().Add(50 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("DownloadBatch() error = %v", err)
	}
	resultByFileID := resultsByFileID(results)
	if resultByFileID[1].ErrorCode != domain.ErrorTimeout {
		t.Fatalf("error code = %q, want %q", resultByFileID[1].ErrorCode, domain.ErrorTimeout)
	}
}

func resultsByFileID(results []FileResult) map[int64]FileResult {
	resultByFileID := make(map[int64]FileResult, len(results))
	for _, result := range results {
		resultByFileID[result.FileID] = result
	}
	return resultByFileID
}
