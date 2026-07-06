INSERT INTO "public"."permissions" (resource, action, name)
VALUES
    ('users', 'create', 'users.create'),
    ('users', 'read', 'users.read'),
    ('users', 'update', 'users.update'),
    ('users', 'delete', 'users.delete')
ON CONFLICT (name) DO NOTHING;

INSERT INTO "public"."role_permissions" ("role_id", "permission_id")
    SELECT "roles"."id" as "role_id", "permissions"."id" as "permission_id" FROM "roles" LEFT JOIN "permissions" ON "resource" = 'users' WHERE "roles"."name" = 'superadmin'
    ON CONFLICT ("role_id", "permission_id") DO NOTHING;
