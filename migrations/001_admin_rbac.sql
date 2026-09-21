-- 后台管理员与 RBAC 初始化脚本
-- 数据库 safewgocp 需要提前手动创建。
-- 本脚本整合自旧版 001_RBAC初始化.sql，并适配当前 Go 模型的表名和字段。

USE `safewgocp`;

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `roles` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL,
  `description` TEXT NULL,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_roles_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='后台角色';

CREATE TABLE IF NOT EXISTS `permissions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(100) NOT NULL,
  `name` VARCHAR(100) NOT NULL,
  `description` TEXT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_permissions_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='后台权限';

CREATE TABLE IF NOT EXISTS `admins` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(64) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  `enabled` TINYINT(1) NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_admins_username` (`username`),
  KEY `idx_admins_role_id` (`role_id`),
  CONSTRAINT `fk_admins_role` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='后台管理员';

CREATE TABLE IF NOT EXISTS `role_permissions` (
  `role_id` BIGINT UNSIGNED NOT NULL,
  `permission_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`role_id`, `permission_id`),
  KEY `idx_role_permissions_permission_id` (`permission_id`),
  CONSTRAINT `fk_role_permissions_role` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_role_permissions_permission` FOREIGN KEY (`permission_id`) REFERENCES `permissions` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色权限关联';

-- 当前基础后台使用的最小权限集合。
INSERT INTO `permissions` (`code`, `name`, `description`) VALUES
  ('dashboard:view', '查看首页', '允许访问后台控制台'),
  ('admin:manage', '管理员管理', '允许新增、编辑、删除管理员'),
  ('role:manage', '角色管理', '允许新增、编辑、删除角色及分配权限'),
  ('permission:view', '查看权限', '允许查看权限列表'),
  ('system:config', '系统配置', '允许查看和修改系统配置'),
  ('lottery:manage', '彩票管理', '允许新增、编辑、删除彩种及管理开奖记录')
ON DUPLICATE KEY UPDATE
  `name` = VALUES(`name`),
  `description` = VALUES(`description`);

-- 旧版脚本中的默认超级管理员角色，适配为当前 roles 表。
INSERT INTO `roles` (`name`, `description`, `created_at`, `updated_at`)
VALUES ('超级管理员', '拥有全部基础权限', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
  `description` = VALUES(`description`),
  `updated_at` = NOW(3);

SET @super_admin_role_id = (
  SELECT `id` FROM `roles` WHERE `name` = '超级管理员' LIMIT 1
);

-- 为超级管理员角色绑定当前系统中的全部权限，可重复执行。
INSERT IGNORE INTO `role_permissions` (`role_id`, `permission_id`)
SELECT @super_admin_role_id, `id`
FROM `permissions`;

-- 默认密码：admin123
-- password_hash 为 bcrypt 哈希；首次登录后请立即修改密码。
INSERT IGNORE INTO `admins` (
  `username`, `password_hash`, `name`, `role_id`, `enabled`, `created_at`, `updated_at`
) VALUES (
  'admin',
  '$2y$10$1z.gPJNCEwnJe9eORaJvu.Dg36OTWOYjKQS/zZeqx/Lsd.1PAWlRW',
  '系统管理员',
  @super_admin_role_id,
  1,
  NOW(3),
  NOW(3)
);
