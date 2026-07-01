INSERT INTO "public"."roles" (name, description, is_system)
VALUES
    ('superadmin', 'Super Administrator', true),
    ('admin', 'Administrator', true),
    ('user', 'Default User', true)
ON CONFLICT (name) DO NOTHING;