package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrInvalidURL   = errors.New("invalid URL")
	ErrFileTooLarge = errors.New("file is too large")
	ErrHTTPStatus   = errors.New("unexpected HTTP status")
)

type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status: %d", e.StatusCode)
}

func (e *HTTPStatusError) Unwrap() error {
	return ErrHTTPStatus
}
