CREATE TABLE "public"."audit_logs" (
    "id" bigserial NOT NULL,
    "resource_name" text NOT NULL,
    "resource_id" text NOT NULL,
    "actor_id" bigint,
    "actor_name" text NOT NULL,
    "actor_type" text,
    "action" text,
    "before" jsonb,
    "after" jsonb,
    "metadata" jsonb,
    "created_at" timestamptz default CURRENT_TIMESTAMP,
    CONSTRAINT "pk_audit_logs_id" PRIMARY KEY ("id")
)