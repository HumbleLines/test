/*
 Navicat Premium Dump SQL

 Source Server         : localhost-pg
 Source Server Type    : PostgreSQL
 Source Server Version : 160010 (160010)
 Source Host           : 127.0.0.1:5432
 Source Catalog        : app
 Source Schema         : public

 Target Server Type    : PostgreSQL
 Target Server Version : 160010 (160010)
 File Encoding         : 65001

 Date: 28/09/2025 14:36:11
*/


-- ----------------------------
-- Table structure for user
-- ----------------------------
DROP TABLE IF EXISTS "public"."user";
CREATE TABLE "public"."user" (
                                 "id" int8 NOT NULL DEFAULT nextval('user_id_seq'::regclass),
                                 "name" varchar(255) COLLATE "pg_catalog"."default" NOT NULL,
                                 "age" int4 NOT NULL,
                                 "email" varchar(255) COLLATE "pg_catalog"."default" NOT NULL,
                                 "tags_json" text COLLATE "pg_catalog"."default",
                                 "created_at" timestamptz(6) NOT NULL DEFAULT now(),
                                 "updated_at" timestamptz(6) NOT NULL DEFAULT now()
)
;

-- ----------------------------
-- Checks structure for table user
-- ----------------------------
ALTER TABLE "public"."user" ADD CONSTRAINT "user_age_check" CHECK (age >= 0 AND age <= 150);

-- ----------------------------
-- Primary Key structure for table user
-- ----------------------------
ALTER TABLE "public"."user" ADD CONSTRAINT "user_pkey" PRIMARY KEY ("id");
