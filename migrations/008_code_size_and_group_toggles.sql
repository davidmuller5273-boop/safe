-- 6码/7码预测：预测表增加 code_size；群设置增加 enable_6_code / enable_7_code。

USE `safewgocp`;

SET NAMES utf8mb4;

ALTER TABLE `hot_number_predictions`
  ADD COLUMN IF NOT EXISTS `code_size` INT NOT NULL DEFAULT 7 COMMENT '预测码数 6 或 7' AFTER `issue_number`;

-- Drop old unique index if present, then create composite including code_size.
-- MySQL 8 may not support IF EXISTS on DROP INDEX in all versions; ignore errors manually if needed.
SET @exist := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'hot_number_predictions'
    AND index_name = 'idx_hot_predictions_lottery_issue'
);
SET @sql := IF(@exist > 0,
  'ALTER TABLE `hot_number_predictions` DROP INDEX `idx_hot_predictions_lottery_issue`',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exist2 := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'hot_number_predictions'
    AND index_name = 'idx_hot_predictions_lottery_issue_size'
);
SET @sql2 := IF(@exist2 = 0,
  'ALTER TABLE `hot_number_predictions` ADD UNIQUE KEY `idx_hot_predictions_lottery_issue_size` (`lottery_type_id`, `issue_number`, `code_size`)',
  'SELECT 1');
PREPARE stmt2 FROM @sql2; EXECUTE stmt2; DEALLOCATE PREPARE stmt2;

ALTER TABLE `bot_group_settings`
  ADD COLUMN IF NOT EXISTS `enable_6_code` TINYINT(1) NOT NULL DEFAULT 0 AFTER `push_enabled`,
  ADD COLUMN IF NOT EXISTS `enable_7_code` TINYINT(1) NOT NULL DEFAULT 0 AFTER `enable_6_code`;
