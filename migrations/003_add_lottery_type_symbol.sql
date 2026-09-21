-- 为已有彩种表补充彩种标识字段。

USE `safewgocp`;

ALTER TABLE `lottery_types`
  ADD COLUMN IF NOT EXISTS `symbol` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '彩种标识' AFTER `name`;
