CREATE SCHEMA IF NOT EXISTS "public";

CREATE TABLE "public"."users" (
    "id" bigserial NOT NULL,
    "username" varchar(100) NOT NULL UNIQUE,
    "password" text NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz,
    "deleted_at" timestamptz,
    CONSTRAINT "pk_users_id" PRIMARY KEY ("id")
);

CREATE INDEX IF NOT EXISTS idx_users_active ON users(id) WHERE deleted_at IS NULL;