-- 1) 用户：存在则更新密码，不存在则创建
CREATE USER IF NOT EXISTS app IDENTIFIED WITH plaintext_password BY 'AppPassw0rd!';
ALTER USER app IDENTIFIED WITH plaintext_password BY 'AppPassw0rd!';

-- 2) 库：确保存在
CREATE DATABASE IF NOT EXISTS app;

-- 3) 权限（开发可用更宽的 *.*；生产建议最小权限）
GRANT ALL ON app.* TO app;

-- 4) 表：与 INSERT 列一一对应
CREATE TABLE IF NOT EXISTS app.trace_test
(
    action     String,
    value      String,
    trace_id   String,
    span_id    String,
    created_at DateTime DEFAULT now()
    )
    ENGINE = MergeTree
    ORDER BY (trace_id, span_id, created_at);
