CREATE TYPE faq_status AS ENUM ('unanswered', 'answered', 'dismissed');

CREATE TABLE "public"."faqs" (
    id                  UUID PRIMARY KEY DEFAULT uuidv7(),
    question            TEXT NOT NULL,
    answer              TEXT,
    status              faq_status NOT NULL DEFAULT 'unanswered',
    asked_by            BIGINT,
    answered_by         BIGINT,
    file_id             UUID NOT NULL REFERENCES files(id),
    answer_content_hash VARCHAR(64),
    last_indexed_hash   VARCHAR(64),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    answered_at         TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_faqs_status ON faqs(status);

-- Dedupe: only one open 'unanswered' row per normalized question.
CREATE UNIQUE INDEX uq_faqs_unanswered_question
    ON faqs (lower(question)) WHERE status = 'unanswered';
