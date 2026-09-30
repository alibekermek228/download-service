package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"internship-download-service/internal/domain"
)

type repositoryStub struct {
	created domain.Download
	failID  int64
}

func (r *repositoryStub) CreateDownload(context.Context, time.Duration, []string) (domain.Download, error) {
	return r.created, nil
}

func (r *repositoryStub) GetDownload(context.Context, int64) (domain.Download, error) {
	return domain.Download{}, domain.ErrNotFound
}

func (r *repositoryStub) GetFile(context.Context, int64, int64) (domain.File, error) {
	return domain.File{}, domain.ErrNotFound
}

func (r *repositoryStub) FailDownload(_ context.Context, id int64, _ string) error {
	r.failID = id
	return nil
}

type starterStub struct {
	startedID int64
	err       error
}

func (s *starterStub) Start(_ context.Context, id int64, _ time.Time) error {
	s.startedID = id
	return s.err
}

func TestCreateDownloadSuccess(t *testing.T) {
	t.Parallel()

	createdAt := time.Now()
	repository := &repositoryStub{created: domain.Download{
		ID: 12, Status: domain.StatusProcess, CreatedAt: createdAt,
	}}
	starter := &starterStub{}
	service := New(repository, starter)

	got, err := service.CreateDownload(
		context.Background(),
		[]string{"https://example.com/file.pdf"},
		time.Minute,
	)
	if err != nil {
		t.Fatalf("CreateDownload() error = %v", err)
	}
	if got.ID != 12 {
		t.Fatalf("CreateDownload() ID = %d, want 12", got.ID)
	}
	if starter.startedID != 12 {
		t.Fatalf("workflow started with ID = %d, want 12", starter.startedID)
	}
}

func TestCreateDownloadInvalidURL(t *testing.T) {
	t.Parallel()

	service := New(&repositoryStub{}, &starterStub{})
	_, err := service.CreateDownload(
		context.Background(),
		[]string{"not-a-url"},
		time.Minute,
	)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("CreateDownload() error = %v, want ErrInvalidInput", err)
	}
}

func TestCreateDownloadWorkflowErrorMarksRequestAsFailed(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{created: domain.Download{
		ID: 15, Status: domain.StatusProcess, CreatedAt: time.Now(),
	}}
	starter := &starterStub{err: errors.New("temporal is unavailable")}
	service := New(repository, starter)

	_, err := service.CreateDownload(
		context.Background(),
		[]string{"https://example.com"},
		time.Minute,
	)
	if err == nil {
		t.Fatal("CreateDownload() error = nil, want error")
	}
	if repository.failID != 15 {
		t.Fatalf("failed download ID = %d, want 15", repository.failID)
	}
}
