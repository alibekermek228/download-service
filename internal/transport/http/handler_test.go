package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"internship-download-service/internal/domain"
)

type serviceStub struct {
	created domain.Download
	err     error
}

func (s *serviceStub) CreateDownload(context.Context, []string, time.Duration) (domain.Download, error) {
	return s.created, s.err
}

func (s *serviceStub) GetDownload(context.Context, int64) (domain.Download, error) {
	return domain.Download{}, s.err
}

func (s *serviceStub) GetFile(context.Context, int64, int64) (domain.File, error) {
	return domain.File{}, s.err
}

func TestCreateDownloadSuccess(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&serviceStub{created: domain.Download{
		ID: 12, Status: domain.StatusProcess,
	}})
	request := httptest.NewRequest(http.MethodPost, "/downloads", strings.NewReader(`{
		"files":[{"url":"https://example.com/file.pdf"}],
		"timeout":"60s"
	}`))
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	var response createDownloadResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 12 || response.Status != domain.StatusProcess {
		t.Fatalf("response = %+v, want id=12 status=PROCESS", response)
	}
}

func TestCreateDownloadInvalidJSON(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&serviceStub{})
	request := httptest.NewRequest(http.MethodPost, "/downloads", strings.NewReader(`{"files":`))
	recorder := httptest.NewRecorder()

	handler.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
