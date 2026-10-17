package main

import (
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	cfg, err := loadConfig(get)
	if err != nil || cfg.Port != "8080" || cfg.DBPath != "data/app.db" || cfg.ExportDir != "exports" || cfg.RequestTimeout != 5*time.Second {
		t.Fatal(cfg, err)
	}
	env["PORT"], env["REQUEST_TIMEOUT"] = "9090", "250ms"
	if cfg, _ = loadConfig(get); cfg.Port != "9090" || cfg.RequestTimeout != 250*time.Millisecond {
		t.Fatal(cfg)
	}
	env["REQUEST_TIMEOUT"] = "abc"
	if _, err = loadConfig(get); err == nil {
		t.Fatal("want error")
	}
}
