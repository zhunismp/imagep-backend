package config

import (
	"testing"
)

func TestValidateServerConfig_AcceptsValidHostAndPort(t *testing.T) {
	cfg := In{ServerHost: "0.0.0.0", ServerPort: 8080}
	if err := validateServerConfig(cfg); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateServerConfig_RejectsPortZero(t *testing.T) {
	cfg := In{ServerHost: "0.0.0.0", ServerPort: 0}
	if err := validateServerConfig(cfg); err == nil {
		t.Error("expected error for port 0")
	}
}

func TestValidateServerConfig_RejectsPortAbove65535(t *testing.T) {
	cfg := In{ServerHost: "0.0.0.0", ServerPort: 70000}
	if err := validateServerConfig(cfg); err == nil {
		t.Error("expected error for port > 65535")
	}
}

func TestValidateServerConfig_RejectsInvalidIPAddress(t *testing.T) {
	cfg := In{ServerHost: "not-an-ip", ServerPort: 8080}
	if err := validateServerConfig(cfg); err == nil {
		t.Error("expected error for invalid IP")
	}
}
