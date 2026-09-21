package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AdminPort, APIPort, ChatPort int
	MySQLHost, MySQLUser         string
	MySQLPassword, MySQLDatabase string
	MySQLPort                    int
	JWTSecret                    string
	ExpireHours                  int
	BSCRPCURL                    string
	RedisAddr, RedisPassword     string
	RedisDB                      int
	SafeWAPIBaseURL              string
	DeveloperUserIDs             []string
}

type fileConfig struct {
	App struct {
		AdminPort int `yaml:"admin_port"`
		APIPort   int `yaml:"api_port"`
		ChatPort  int `yaml:"chat_port"`
	} `yaml:"app"`
	MySQL struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
	} `yaml:"mysql"`
	Auth struct {
		JWTSecret   string `yaml:"jwt_secret"`
		ExpireHours int    `yaml:"expire_hours"`
	} `yaml:"auth"`
	Lottery struct {
		BSCRPCURL string `yaml:"bsc_rpc_url"`
	} `yaml:"lottery"`
	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
	SafeW struct {
		APIBaseURL string `yaml:"api_base_url"`
	} `yaml:"safew"`
	Bot struct {
		DeveloperUserIDs string `yaml:"developer_user_ids"`
	} `yaml:"bot"`
}

func Load() (Config, error) {
	file := fileConfig{}
	file.App.AdminPort, file.App.APIPort, file.App.ChatPort = 8081, 8080, 8082
	file.MySQL.Host, file.MySQL.Port = "127.0.0.1", 3306
	file.MySQL.User, file.MySQL.Password, file.MySQL.Database = "safe", "CHANGE_ME", "safe"
	file.Auth.JWTSecret, file.Auth.ExpireHours = "change-me-in-production", 24
	file.Lottery.BSCRPCURL = "https://bsc-dataseed.bnbchain.org"
	file.Redis.Addr = "127.0.0.1:6379"
	file.SafeW.APIBaseURL = "https://api.safew.bot"

	path := env("CONFIG_FILE", "config/app.yml")
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &file); err != nil {
			return Config{}, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	devIDs := env("DEVELOPER_USER_IDS", file.Bot.DeveloperUserIDs)
	return Config{
		AdminPort: envInt("ADMIN_PORT", file.App.AdminPort), APIPort: envInt("API_PORT", file.App.APIPort), ChatPort: envInt("CHAT_PORT", file.App.ChatPort),
		MySQLHost: env("MYSQL_HOST", file.MySQL.Host), MySQLPort: envInt("MYSQL_PORT", file.MySQL.Port),
		MySQLUser: env("MYSQL_USER", file.MySQL.User), MySQLPassword: env("MYSQL_PASSWORD", file.MySQL.Password),
		MySQLDatabase: env("MYSQL_DATABASE", file.MySQL.Database),
		JWTSecret:     env("JWT_SECRET", file.Auth.JWTSecret), ExpireHours: envInt("JWT_EXPIRE_HOURS", file.Auth.ExpireHours),
		BSCRPCURL: env("BSC_RPC_URL", file.Lottery.BSCRPCURL),
		RedisAddr: env("REDIS_ADDR", file.Redis.Addr), RedisPassword: env("REDIS_PASSWORD", file.Redis.Password), RedisDB: envInt("REDIS_DB", file.Redis.DB),
		SafeWAPIBaseURL:  env("SAFEW_API_BASE_URL", file.SafeW.APIBaseURL),
		DeveloperUserIDs: splitCSV(devIDs),
	}, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err == nil {
		return value
	}
	return fallback
}
