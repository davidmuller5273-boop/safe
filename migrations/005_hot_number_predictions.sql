-- 七位热号预测及开奖后评估记录。

USE `safewgocp`;

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `hot_number_predictions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `lottery_type_id` BIGINT UNSIGNED NOT NULL COMMENT '彩种 ID',
  `issue_number` VARCHAR(64) NOT NULL COMMENT '预测期号',
  `hot_numbers` TEXT NOT NULL COMMENT '按新到旧排列的七个热号 JSON',
  `prediction` VARCHAR(16) NOT NULL COMMENT '升序排列的七位预测值',
  `actual_hot_number` VARCHAR(2) NULL COMMENT '本期实际第一名，10 映射为 0',
  `correct` TINYINT(1) NULL COMMENT '预测是否命中',
  `evaluated_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_hot_predictions_lottery_issue` (`lottery_type_id`, `issue_number`),
  KEY `idx_hot_number_predictions_lottery_type_id` (`lottery_type_id`),
  CONSTRAINT `fk_hot_number_predictions_lottery_type`
    FOREIGN KEY (`lottery_type_id`) REFERENCES `lottery_types` (`id`)
    ON UPDATE CASCADE ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='热号预测记录';
