-- 亚军（position=2）热号预测：预测表增加 position；群设置增加 enable_runner_up_6/7_code。
-- 正常情况下无需手工执行：database.Open / ensure*Schema 会在启动时幂等补齐。

USE `safewgocp`;

SET NAMES utf8mb4;

ALTER TABLE `hot_number_predictions`
  ADD COLUMN IF NOT EXISTS `position` INT NOT NULL DEFAULT 1 COMMENT '名次 1=冠军 2=亚军' AFTER `code_size`;

UPDATE `hot_number_predictions` SET `position` = 1 WHERE `position` = 0 OR `position` IS NULL;

-- Drop unique index that omits position, then create composite including position.
SET @exist := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'hot_number_predictions'
    AND index_name = 'idx_hot_predictions_lottery_issue_size'
);
SET @sql := IF(@exist > 0,
  'ALTER TABLE `hot_number_predictions` DROP INDEX `idx_hot_predictions_lottery_issue_size`',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exist2 := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'hot_number_predictions'
    AND index_name = 'idx_hot_predictions_lottery_issue_size_pos'
);
SET @sql2 := IF(@exist2 = 0,
  'ALTER TABLE `hot_number_predictions` ADD UNIQUE KEY `idx_hot_predictions_lottery_issue_size_pos` (`lottery_type_id`, `issue_number`, `code_size`, `position`)',
  'SELECT 1');
PREPARE stmt2 FROM @sql2; EXECUTE stmt2; DEALLOCATE PREPARE stmt2;

ALTER TABLE `bot_group_settings`
  ADD COLUMN IF NOT EXISTS `enable_runner_up_6_code` TINYINT(1) NOT NULL DEFAULT 0 AFTER `enable_7_code`,
  ADD COLUMN IF NOT EXISTS `enable_runner_up_7_code` TINYINT(1) NOT NULL DEFAULT 0 AFTER `enable_runner_up_6_code`;
