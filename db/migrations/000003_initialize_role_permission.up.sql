ALTER TABLE "public"."permissions" ADD "resource" VARCHAR(50) NOT NULL;
ALTER TABLE "public"."permissions" ADD "action" VARCHAR(50) NOT NULL;
ALTER TABLE "public"."permissions" ADD "name" VARCHAR(50) NOT NULL;

ALTER TABLE "public"."permissions"
ADD CONSTRAINT "uq_permissions_name"
UNIQUE (name);

ALTER TABLE "public"."permissions"
ADD CONSTRAINT "uq_permissions_resource_action"
UNIQUE ("resource", "action");

CREATE TABLE "public"."role_permissions" (
    "role_id" bigint NOT NULL,
    "permission_id" bigint NOT NULL,
    CONSTRAINT "pk_role_permissions_id" PRIMARY KEY ("role_id", "permission_id"),
    CONSTRAINT "fk_role_id" FOREIGN KEY ("role_id") REFERENCES roles("id") ON DELETE CASCADE,
    CONSTRAINT "fk_permission_id" FOREIGN KEY ("permission_id") REFERENCES permissions("id") ON DELETE CASCADE
);
