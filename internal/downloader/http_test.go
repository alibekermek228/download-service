package downloader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"internship-download-service/internal/domain"
)

func TestHTTPDownloaderSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	d := NewHTTP(100)
	got, err := d.Download(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("Download() = %q, want hello", string(got))
	}
}

func TestHTTPDownloaderFileTooLarge(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("too large"))
	}))
	defer server.Close()

	d := NewHTTP(3)
	_, err := d.Download(context.Background(), server.URL)
	if !errors.Is(err, domain.ErrFileTooLarge) {
		t.Fatalf("Download() error = %v, want ErrFileTooLarge", err)
	}
}

func TestHTTPDownloaderHTTPStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	d := NewHTTP(100)
	_, err := d.Download(context.Background(), server.URL)
	var statusError *domain.HTTPStatusError
	if !errors.As(err, &statusError) {
		t.Fatalf("Download() error = %v, want HTTPStatusError", err)
	}
	if statusError.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", statusError.StatusCode, http.StatusNotFound)
	}
}
