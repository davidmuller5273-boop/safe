CREATE TABLE IF NOT EXISTS bot_group_settings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  chat_id VARCHAR(64) NOT NULL,
  push_enabled TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  UNIQUE KEY idx_bot_group_settings_chat_id (chat_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
