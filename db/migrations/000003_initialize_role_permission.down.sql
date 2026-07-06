DROP TABLE "public"."role_permissions";

ALTER TABLE "public"."permissions" DROP "resource";
ALTER TABLE "public"."permissions" DROP "action";
ALTER TABLE "public"."permissions" DROP "name";
