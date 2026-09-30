package temporalapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"internship-download-service/internal/domain"
)

type activityRepositoryStub struct {
	files   []domain.File
	mutex   sync.Mutex
	results map[int64]fileResult
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
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.results == nil {
		r.results = make(map[int64]fileResult)
	}
	r.results[fileID] = fileResult{fileID: fileID, data: data, errorCode: errorCode}
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

	err := activities.DownloadBatch(context.Background(), WorkflowInput{
		DownloadID: 10,
		Deadline:   time.Now().Add(time.Second),
	})
	if err != nil {
		t.Fatalf("DownloadBatch() error = %v", err)
	}
	if string(repository.results[1].data) != "ok" {
		t.Fatalf("successful file data = %q, want ok", repository.results[1].data)
	}
	if repository.results[2].errorCode != domain.ErrorDownload {
		t.Fatalf("failed file code = %q, want %q", repository.results[2].errorCode, domain.ErrorDownload)
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

	err := activities.DownloadBatch(context.Background(), WorkflowInput{
		DownloadID: 11,
		Deadline:   time.Now().Add(50 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("DownloadBatch() error = %v", err)
	}
	if repository.results[1].errorCode != domain.ErrorTimeout {
		t.Fatalf("error code = %q, want %q", repository.results[1].errorCode, domain.ErrorTimeout)
	}
}
