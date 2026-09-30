package usecase

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"internship-download-service/internal/domain"
)

const (
	maxFiles          = 20
	maxRequestTimeout = 5 * time.Minute
)

type Repository interface {
	CreateDownload(ctx context.Context, timeout time.Duration, urls []string) (domain.Download, error)
	GetDownload(ctx context.Context, id int64) (domain.Download, error)
	GetFile(ctx context.Context, downloadID, fileID int64) (domain.File, error)
	FailDownload(ctx context.Context, id int64, errorCode string) error
}

type WorkflowStarter interface {
	Start(ctx context.Context, downloadID int64, deadline time.Time) error
}

type Service struct {
	repository Repository
	starter    WorkflowStarter
}

func New(repository Repository, starter WorkflowStarter) *Service {
	return &Service{repository: repository, starter: starter}
}

func (s *Service) CreateDownload(
	ctx context.Context,
	urls []string,
	timeout time.Duration,
) (domain.Download, error) {
	if err := validateInput(urls, timeout); err != nil {
		return domain.Download{}, err
	}

	download, err := s.repository.CreateDownload(ctx, timeout, urls)
	if err != nil {
		return domain.Download{}, fmt.Errorf("create download: %w", err)
	}

	deadline := download.CreatedAt.Add(timeout)
	if err := s.starter.Start(ctx, download.ID, deadline); err != nil {
		// The request is already stored. Mark it as completed with an error so it
		// does not remain in PROCESS forever when Temporal cannot start a workflow.
		cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.repository.FailDownload(cleanupContext, download.ID, domain.ErrorWorkflow)
		return domain.Download{}, fmt.Errorf("start workflow: %w", err)
	}

	return download, nil
}

func (s *Service) GetDownload(ctx context.Context, id int64) (domain.Download, error) {
	download, err := s.repository.GetDownload(ctx, id)
	if err != nil {
		return domain.Download{}, fmt.Errorf("get download: %w", err)
	}
	return download, nil
}

func (s *Service) GetFile(ctx context.Context, downloadID, fileID int64) (domain.File, error) {
	file, err := s.repository.GetFile(ctx, downloadID, fileID)
	if err != nil {
		return domain.File{}, fmt.Errorf("get file: %w", err)
	}
	return file, nil
}

func validateInput(urls []string, timeout time.Duration) error {
	if len(urls) == 0 {
		return fmt.Errorf("%w: files list must not be empty", domain.ErrInvalidInput)
	}
	if len(urls) > maxFiles {
		return fmt.Errorf("%w: no more than %d files are allowed", domain.ErrInvalidInput, maxFiles)
	}
	if timeout <= 0 || timeout > maxRequestTimeout {
		return fmt.Errorf("%w: timeout must be between 1ns and %s", domain.ErrInvalidInput, maxRequestTimeout)
	}

	for _, rawURL := range urls {
		parsed, err := url.ParseRequestURI(rawURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("%w: only absolute http and https URLs are allowed", domain.ErrInvalidInput)
		}
	}

	return nil
}
