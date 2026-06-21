-- 给 app 用户授权到 xxl-job 库
GRANT ALL ON `xxl-job`.* TO 'app'@'%';
CREATE DATABASE IF NOT EXISTS nacos_config DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_general_ci;
CREATE USER IF NOT EXISTS 'app'@'%' IDENTIFIED BY 'AppPassw0rd!'
-- 授权 nacos_config 库的全部权限
GRANT ALL PRIVILEGES ON nacos_config.* TO 'app'@'%';
FLUSH PRIVILEGES;


CREATE TABLE `user` (
`id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键',
`name` varchar(255) NOT NULL COMMENT '姓名',
`age` int(11) NOT NULL COMMENT '年龄 0~150',
`email` varchar(255) NOT NULL COMMENT '邮箱',
`tags_json` text COMMENT 'tags 的 JSON 串',
`created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
`updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=29 DEFAULT CHARSET=utf8mb4;
