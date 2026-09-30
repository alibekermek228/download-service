CREATE TABLE IF NOT EXISTS downloads (
    id BIGSERIAL PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('PROCESS', 'DONE')),
    timeout_ms BIGINT NOT NULL CHECK (timeout_ms > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS download_files (
    id BIGSERIAL PRIMARY KEY,
    download_id BIGINT NOT NULL REFERENCES downloads(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    url TEXT NOT NULL,
    data BYTEA,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (download_id, position),
    CHECK (NOT (data IS NOT NULL AND error_code IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS download_files_download_id_idx
    ON download_files(download_id);
