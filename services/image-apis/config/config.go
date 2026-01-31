package config

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sethvargo/go-envconfig"
)

type ServerCfg struct {
	Host string
	Port int
}

type PubsubCfg struct {
	KafkaAddress string
}

type CacheCfg struct {
	RedisAddress  string
	RedisPassword string
	RedisDB       int
}

type AppCfg struct {
	Name            string
	Environment     string
	BaseApiPrefix   string
	MaxFiles        int
	PollingInterval int
}

type Config struct {
	ServerCfg
	PubsubCfg
	CacheCfg
	AppCfg
}

type In struct {
	ServerHost string `env:"HOST, default=0.0.0.0"`
	ServerPort int    `env:"PORT, default=8080"`

	KafkaAddress string `env:"KAFKA_ADDRESS, default=localhost:9092"`

	RedisAddress  string `env:"REDIS_ADDRESS, default=localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD, default=secret"`
	RedisDB       int    `env:"REDIS_DB, default=0"`

	AppName            string `env:"APP_NAME, default=image-apis"`
	AppEnvironment     string `env:"ENVIRONMENT, default=development"`
	AppBaseApiPrefix   string `env:"BASE_API_PREFIX, default=/api/v1"`
	AppMaxFiles        int    `env:"MAX_FILES, default=100"`
	AppPollingInterval int    `env:"POLLING_INTERVAL, default=2000"`
}

func LoadCfg(ctx context.Context) (Config, error) {
	var input In

	c, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := envconfig.Process(c, &input); err != nil {
		return Config{}, err
	}

	if err := validateServerConfig(input); err != nil {
		return Config{}, err
	}

	cfg := Config{
		ServerCfg: ServerCfg{
			Host: input.ServerHost,
			Port: input.ServerPort,
		},
		PubsubCfg: PubsubCfg{
			KafkaAddress: input.KafkaAddress,
		},
		CacheCfg: CacheCfg{
			RedisAddress:  input.RedisAddress,
			RedisPassword: input.RedisPassword,
			RedisDB:       input.RedisDB,
		},
		AppCfg: AppCfg{
			Name:            input.AppName,
			Environment:     input.AppEnvironment,
			BaseApiPrefix:   input.AppBaseApiPrefix,
			MaxFiles:        input.AppMaxFiles,
			PollingInterval: input.AppPollingInterval,
		},
	}

	return cfg, nil
}

func validateServerConfig(cfg In) error {
	if cfg.ServerPort < 1 || cfg.ServerPort > 65535 {
		return fmt.Errorf("expected port to be between 1 and 65535 but received: %d", cfg.ServerPort)
	}

	hostIP := net.ParseIP(cfg.ServerHost)
	if hostIP == nil {
		return fmt.Errorf("expected valid IPv4 address but received: %v", hostIP)
	}

	return nil
}
