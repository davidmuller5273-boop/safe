package cmd

import (
	"github.com/spf13/cobra"
	"github.com/davidmuller5273-boop/safe/services/admin"
	"github.com/davidmuller5273-boop/safe/services/api"
	"github.com/davidmuller5273-boop/safe/services/chat"
	lotterycollector "github.com/davidmuller5273-boop/safe/services/lottery_collector"
	safewbot "github.com/davidmuller5273-boop/safe/services/safew_bot"
	"github.com/davidmuller5273-boop/safe/services/task"
)

var rootCmd = &cobra.Command{Use: "safe", Short: "SafeW/SafeX 聊天机器人与管理后台"}

func Execute() error { return rootCmd.Execute() }

func init() {
	rootCmd.AddCommand(
		&cobra.Command{Use: "admin", Short: "启动后台管理服务", RunE: func(*cobra.Command, []string) error { return admin.Run() }},
		&cobra.Command{Use: "api", Short: "启动 API 空服务", RunE: func(*cobra.Command, []string) error { return api.Run() }},
		&cobra.Command{Use: "chat", Short: "启动聊天空服务", RunE: func(*cobra.Command, []string) error { return chat.Run() }},
		&cobra.Command{Use: "task", Short: "启动任务空进程", RunE: func(*cobra.Command, []string) error { return task.Run() }},
		&cobra.Command{Use: "lottery-collector", Short: "启动彩票采集任务", RunE: func(*cobra.Command, []string) error { return lotterycollector.Run() }},
		&cobra.Command{Use: "safew-bot", Short: "启动 SafeW 机器人消息服务", RunE: func(*cobra.Command, []string) error { return safewbot.Run() }},
	)
}
