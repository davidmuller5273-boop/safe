package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	content := []byte("mysql:\n  host: 127.0.0.1\n  port: 3307\n  user: root\n  password: root\n  database: safewgocp\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQLPort != 3307 {
		t.Fatalf("MySQLPort = %d, want 3307", cfg.MySQLPort)
	}
	if cfg.MySQLDatabase != "safewgocp" {
		t.Fatalf("MySQLDatabase = %q, want safewgocp", cfg.MySQLDatabase)
	}
}

func TestEnvironmentOverridesYAML(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yml"))
	t.Setenv("MYSQL_PORT", "3310")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQLPort != 3310 {
		t.Fatalf("MySQLPort = %d, want environment override 3310", cfg.MySQLPort)
	}
}
