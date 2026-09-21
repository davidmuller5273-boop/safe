-- 彩种及开奖记录表。
-- 开奖时间使用 Unix 秒时间戳，便于任务服务进行时间比较和调度。

USE `safewgocp`;

SET NAMES utf8mb4;

INSERT INTO `permissions` (`code`, `name`, `description`) VALUES
  ('lottery:manage', '彩票管理', '允许新增、编辑、删除彩种及管理开奖记录')
ON DUPLICATE KEY UPDATE
  `name` = VALUES(`name`),
  `description` = VALUES(`description`);

INSERT IGNORE INTO `role_permissions` (`role_id`, `permission_id`)
SELECT `roles`.`id`, `permissions`.`id`
FROM `roles`
JOIN `permissions` ON `permissions`.`code` = 'lottery:manage'
WHERE `roles`.`name` = '超级管理员';

CREATE TABLE IF NOT EXISTS `lottery_types` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(100) NOT NULL COMMENT '彩种名称',
  `symbol` VARCHAR(64) NOT NULL COMMENT '彩种标识',
  `seconds_per_issue` BIGINT UNSIGNED NOT NULL COMMENT '每期开奖间隔秒数',
  `issues_per_day` BIGINT UNSIGNED NOT NULL COMMENT '每天期数',
  `draws_all_day` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否全天开奖',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_lottery_types_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='彩种';

CREATE TABLE IF NOT EXISTS `draw_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `lottery_type_id` BIGINT UNSIGNED NOT NULL COMMENT '彩种 ID',
  `issue_number` VARCHAR(64) NOT NULL COMMENT '期号',
  `draw_result` TEXT NOT NULL COMMENT '开奖结果字符串',
  `draw_timestamp` BIGINT NOT NULL COMMENT '开奖时间，Unix 秒时间戳',
  `next_draw_timestamp` BIGINT NOT NULL COMMENT '下一期开奖时间，Unix 秒时间戳',
  `next_issue_number` VARCHAR(64) NOT NULL COMMENT '下一期开奖期号',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_draw_records_lottery_issue` (`lottery_type_id`, `issue_number`),
  KEY `idx_draw_records_lottery_type_id` (`lottery_type_id`),
  KEY `idx_draw_records_draw_timestamp` (`draw_timestamp`),
  KEY `idx_draw_records_next_draw_timestamp` (`next_draw_timestamp`),
  CONSTRAINT `fk_draw_records_lottery_type`
    FOREIGN KEY (`lottery_type_id`) REFERENCES `lottery_types` (`id`)
    ON UPDATE CASCADE ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='开奖记录';
