package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/davidmuller5273-boop/safe/internal/config"
	"github.com/davidmuller5273-boop/safe/internal/platform/database"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "初始化或增量更新数据库结构",
	Long: `连接已存在的 MySQL 数据库，补充缺失的后台管理表、字段和索引，
并以幂等方式初始化基础权限数据。命令不会删除表、清空数据或覆盖已有管理员密码。`,
	RunE: func(*cobra.Command, []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("🔧 开始检查 MySQL 数据库 %s@%s:%d/%s\n", cfg.MySQLUser, cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDatabase)

		if _, err := database.Open(
			cfg.MySQLHost,
			cfg.MySQLPort,
			cfg.MySQLUser,
			cfg.MySQLPassword,
			cfg.MySQLDatabase,
		); err != nil {
			return fmt.Errorf("数据库初始化失败: %w", err)
		}

		fmt.Println("✅ 数据库结构和基础权限数据检查完成")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
