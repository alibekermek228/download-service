package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"internship-download-service/internal/domain"
)

type DownloadRepository struct {
	pool *pgxpool.Pool
}

func NewDownloadRepository(pool *pgxpool.Pool) *DownloadRepository {
	return &DownloadRepository{pool: pool}
}

func (r *DownloadRepository) CreateDownload(
	ctx context.Context,
	timeout time.Duration,
	urls []string,
) (domain.Download, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Download{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	download := domain.Download{
		Status:  domain.StatusProcess,
		Timeout: timeout,
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO downloads (status, timeout_ms)
		VALUES ($1, $2)
		RETURNING id, created_at
	`, download.Status, timeout.Milliseconds()).Scan(&download.ID, &download.CreatedAt)
	if err != nil {
		return domain.Download{}, fmt.Errorf("insert download: %w", err)
	}

	download.Files = make([]domain.File, 0, len(urls))
	for position, rawURL := range urls {
		file := domain.File{
			DownloadID: download.ID,
			Position:   position,
			URL:        rawURL,
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO download_files (download_id, position, url)
			VALUES ($1, $2, $3)
			RETURNING id
		`, download.ID, position, rawURL).Scan(&file.ID)
		if err != nil {
			return domain.Download{}, fmt.Errorf("insert download file: %w", err)
		}
		download.Files = append(download.Files, file)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Download{}, fmt.Errorf("commit transaction: %w", err)
	}
	return download, nil
}

func (r *DownloadRepository) GetDownload(ctx context.Context, id int64) (domain.Download, error) {
	var download domain.Download
	var timeoutMilliseconds int64
	err := r.pool.QueryRow(ctx, `
		SELECT id, status, timeout_ms, created_at
		FROM downloads
		WHERE id = $1
	`, id).Scan(&download.ID, &download.Status, &timeoutMilliseconds, &download.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Download{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Download{}, fmt.Errorf("select download: %w", err)
	}
	download.Timeout = time.Duration(timeoutMilliseconds) * time.Millisecond

	rows, err := r.pool.Query(ctx, `
		SELECT id, download_id, position, url, data, COALESCE(error_code, '')
		FROM download_files
		WHERE download_id = $1
		ORDER BY position
	`, id)
	if err != nil {
		return domain.Download{}, fmt.Errorf("select download files: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var file domain.File
		if err := rows.Scan(
			&file.ID,
			&file.DownloadID,
			&file.Position,
			&file.URL,
			&file.Data,
			&file.ErrorCode,
		); err != nil {
			return domain.Download{}, fmt.Errorf("scan download file: %w", err)
		}
		download.Files = append(download.Files, file)
	}
	if err := rows.Err(); err != nil {
		return domain.Download{}, fmt.Errorf("iterate download files: %w", err)
	}

	return download, nil
}

func (r *DownloadRepository) GetFile(ctx context.Context, downloadID, fileID int64) (domain.File, error) {
	var file domain.File
	err := r.pool.QueryRow(ctx, `
		SELECT id, download_id, position, url, data, COALESCE(error_code, '')
		FROM download_files
		WHERE id = $1 AND download_id = $2 AND data IS NOT NULL
	`, fileID, downloadID).Scan(
		&file.ID,
		&file.DownloadID,
		&file.Position,
		&file.URL,
		&file.Data,
		&file.ErrorCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("select file: %w", err)
	}
	return file, nil
}

func (r *DownloadRepository) ListPendingFiles(ctx context.Context, downloadID int64) ([]domain.File, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, download_id, position, url
		FROM download_files
		WHERE download_id = $1 AND data IS NULL AND error_code IS NULL
		ORDER BY position
	`, downloadID)
	if err != nil {
		return nil, fmt.Errorf("select pending files: %w", err)
	}
	defer rows.Close()

	var files []domain.File
	for rows.Next() {
		var file domain.File
		if err := rows.Scan(&file.ID, &file.DownloadID, &file.Position, &file.URL); err != nil {
			return nil, fmt.Errorf("scan pending file: %w", err)
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending files: %w", err)
	}
	return files, nil
}

func (r *DownloadRepository) SaveFileResult(
	ctx context.Context,
	downloadID, fileID int64,
	data []byte,
	errorCode string,
) error {
	if errorCode == "" {
		commandTag, err := r.pool.Exec(ctx, `
			UPDATE download_files
			SET data = $1, error_code = NULL, updated_at = NOW()
			WHERE id = $2 AND download_id = $3
		`, data, fileID, downloadID)
		if err != nil {
			return fmt.Errorf("update successful file: %w", err)
		}
		if commandTag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	}

	commandTag, err := r.pool.Exec(ctx, `
		UPDATE download_files
		SET data = NULL, error_code = $1, updated_at = NOW()
		WHERE id = $2 AND download_id = $3
	`, errorCode, fileID, downloadID)
	if err != nil {
		return fmt.Errorf("update failed file: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *DownloadRepository) MarkDone(ctx context.Context, downloadID int64) error {
	commandTag, err := r.pool.Exec(ctx, `
		UPDATE downloads
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, domain.StatusDone, downloadID)
	if err != nil {
		return fmt.Errorf("mark download done: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *DownloadRepository) FailDownload(ctx context.Context, downloadID int64, errorCode string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin fail transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE download_files
		SET error_code = $1, updated_at = NOW()
		WHERE download_id = $2 AND data IS NULL AND error_code IS NULL
	`, errorCode, downloadID); err != nil {
		return fmt.Errorf("fail pending files: %w", err)
	}

	commandTag, err := tx.Exec(ctx, `
		UPDATE downloads
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, domain.StatusDone, downloadID)
	if err != nil {
		return fmt.Errorf("fail download: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit fail transaction: %w", err)
	}
	return nil
}
