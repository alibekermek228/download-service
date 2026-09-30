package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"internship-download-service/internal/domain"
)

const maxRequestBodySize = 1024 * 1024

type DownloadService interface {
	CreateDownload(ctx context.Context, urls []string, timeout time.Duration) (domain.Download, error)
	GetDownload(ctx context.Context, id int64) (domain.Download, error)
	GetFile(ctx context.Context, downloadID, fileID int64) (domain.File, error)
}

type Handler struct {
	service DownloadService
}

func NewHandler(service DownloadService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /downloads", h.createDownload)
	mux.HandleFunc("GET /downloads/{id}", h.getDownload)
	mux.HandleFunc("GET /downloads/{id}/files/{file_id}", h.getFile)
	return RequestID(Recovery(mux))
}

type createDownloadRequest struct {
	Files   []createFileRequest `json:"files"`
	Timeout string              `json:"timeout"`
}

type createFileRequest struct {
	URL string `json:"url"`
}

type createDownloadResponse struct {
	ID     int64                 `json:"id"`
	Status domain.DownloadStatus `json:"status"`
}

type downloadResponse struct {
	ID     int64                 `json:"id"`
	Status domain.DownloadStatus `json:"status"`
	Files  []fileResponse        `json:"files"`
}

type fileResponse struct {
	URL    string        `json:"url"`
	FileID *int64        `json:"file_id,omitempty"`
	Error  *errorDetails `json:"error,omitempty"`
}

type errorResponse struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) createDownload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request createDownloadRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body is invalid")
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body must contain one JSON object")
		return
	}

	timeout, err := time.ParseDuration(request.Timeout)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_TIMEOUT", "timeout must be a Go duration such as 60s")
		return
	}

	urls := make([]string, 0, len(request.Files))
	for _, file := range request.Files {
		urls = append(urls, file.URL)
	}

	download, err := h.service.CreateDownload(r.Context(), urls, timeout)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, createDownloadResponse{
		ID:     download.ID,
		Status: download.Status,
	})
}

func (h *Handler) getDownload(w http.ResponseWriter, r *http.Request) {
	id, err := parsePositiveID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "download id must be a positive integer")
		return
	}

	download, err := h.service.GetDownload(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response := downloadResponse{
		ID:     download.ID,
		Status: download.Status,
		Files:  make([]fileResponse, 0, len(download.Files)),
	}
	for _, file := range download.Files {
		item := fileResponse{URL: file.URL}
		switch {
		case file.ErrorCode != "":
			item.Error = &errorDetails{Code: file.ErrorCode}
		case file.Data != nil:
			fileID := file.ID
			item.FileID = &fileID
		}
		response.Files = append(response.Files, item)
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) getFile(w http.ResponseWriter, r *http.Request) {
	downloadID, err := parsePositiveID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "download id must be a positive integer")
		return
	}
	fileID, err := parsePositiveID(r.PathValue("file_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_FILE_ID", "file id must be a positive integer")
		return
	}

	file, err := h.service.GetFile(r.Context(), downloadID, fileID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func parsePositiveID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("extra JSON value")
	}
	return err
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource was not found")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorDetails{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
