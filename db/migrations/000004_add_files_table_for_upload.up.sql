CREATE TYPE upload_status AS ENUM ('pending', 'completed', 'failed');

CREATE TABLE "public"."files" (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id BIGINT NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    s3_bucket VARCHAR(100) NOT NULL,
    s3_key VARCHAR(500) NOT NULL UNIQUE,
    status upload_status NOT NULL DEFAULT 'pending',
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_files_user_id ON files(user_id);
CREATE INDEX idx_files_status ON files(status) WHERE status = 'pending';
