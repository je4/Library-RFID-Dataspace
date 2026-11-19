package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Default(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("failed to change directory to project root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	conf, err := LoadConfig("")
	if err != nil {
		t.Fatalf("expected embedded default config to load, got error: %v", err)
	}
	if conf.Name != "rfid-dataspace-01" {
		t.Errorf("expected client name 'rfid-dataspace-01', got '%s'", conf.Name)
	}
	if conf.Addr != "localhost:5000" {
		t.Errorf("expected addr 'localhost:5000', got '%s'", conf.Addr)
	}
	if conf.DefaultISIL != "CH-001596-0" {
		t.Errorf("expected default_isil 'CH-001596-0', got '%s'", conf.DefaultISIL)
	}
	if len(conf.Channels) == 0 || conf.Channels[0] != "mediathek01" {
		t.Errorf("expected channels ['mediathek01'], got %v", conf.Channels)
	}
}

func TestLoadConfig_CustomFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom.toml")

	tomlData := `
addr = "localhost:9999"
extaddr = "http://localhost:9999"
name = "custom-dataspace"
wsbroker = "wss://broker.example.com:8765"
channels = ["custom-chan"]
max_tags = 250
`
	if err := os.WriteFile(configPath, []byte(tomlData), 0644); err != nil {
		t.Fatalf("failed to write custom config file: %v", err)
	}

	conf, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load custom config file: %v", err)
	}

	if conf.Name != "custom-dataspace" {
		t.Errorf("expected 'custom-dataspace', got '%s'", conf.Name)
	}
	if conf.Addr != "localhost:9999" {
		t.Errorf("expected 'localhost:9999', got '%s'", conf.Addr)
	}
	if conf.MaxTags != 250 {
		t.Errorf("expected max_tags 250, got %d", conf.MaxTags)
	}
}
