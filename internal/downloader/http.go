package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"internship-download-service/internal/domain"
)

type HTTPDownloader struct {
	client      *http.Client
	maxFileSize int64
}

func NewHTTP(maxFileSize int64) *HTTPDownloader {
	return &HTTPDownloader{
		client: &http.Client{
			// The whole request timeout is controlled by the context. This value
			// protects a request if the caller accidentally forgets a deadline.
			Timeout: 10 * time.Minute,
		},
		maxFileSize: maxFileSize,
	}
}

func (d *HTTPDownloader) Download(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidURL, rawURL)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	response, err := d.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &domain.HTTPStatusError{StatusCode: response.StatusCode}
	}
	if response.ContentLength > d.maxFileSize {
		return nil, fmt.Errorf("%w: limit is %d bytes", domain.ErrFileTooLarge, d.maxFileSize)
	}

	limited := io.LimitReader(response.Body, d.maxFileSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if int64(len(data)) > d.maxFileSize {
		return nil, fmt.Errorf("%w: limit is %d bytes", domain.ErrFileTooLarge, d.maxFileSize)
	}

	return data, nil
}

func ErrorCode(err error) string {
	var statusError *domain.HTTPStatusError
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return domain.ErrorTimeout
	case errors.Is(err, domain.ErrInvalidURL):
		return domain.ErrorInvalidURL
	case errors.As(err, &statusError):
		return domain.ErrorHTTPStatus
	case errors.Is(err, domain.ErrFileTooLarge):
		return domain.ErrorFileTooLarge
	default:
		return domain.ErrorDownload
	}
}
