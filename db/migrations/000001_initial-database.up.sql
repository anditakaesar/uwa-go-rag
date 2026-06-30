CREATE SCHEMA IF NOT EXISTS "public";

CREATE TABLE "public"."roles" (
    "id" bigserial NOT NULL,
    "name" varchar(100) NOT NULL UNIQUE,
    "description" text,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "is_system" boolean NOT NULL DEFAULT false,
    CONSTRAINT "pk_roles_id" PRIMARY KEY ("id")
);

CREATE TABLE "public"."users" (
    "id" bigserial NOT NULL,
    "username" varchar(100) NOT NULL UNIQUE,
    "password" text NOT NULL,
    "email" varchar(200) UNIQUE,
    "role_id" bigint NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamptz,
    CONSTRAINT "pk_users_id" PRIMARY KEY ("id"),
    CONSTRAINT "fk_users_role" FOREIGN KEY ("role_id") REFERENCES roles("id")
);

CREATE INDEX "idx_users_username_active" ON users(username) WHERE deleted_at IS NULL;

CREATE TABLE "public"."permissions" (
    "id" bigserial NOT NULL,
    -- reserve the table
    CONSTRAINT "pk_permissions_id" PRIMARY KEY ("id")
);