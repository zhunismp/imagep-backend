package config

import (
	"context"

	"github.com/sethvargo/go-envconfig"
)

type PubsubCfg struct {
	KafkaAddress       string
	KafkaConsumeTopic  string
	KafkaConsumerGroup string
}

type CacheCfg struct {
	RedisAddress  string
	RedisPassword string
	RedisDB       int
}

type AppCfg struct {
	Name        string
	Environment string
}

type Config struct {
	PubsubCfg
	CacheCfg
	AppCfg
}

type In struct {
	KafkaAddress       string `env:"KAFKA_ADDRESS, default=localhost:9092"`
	KafkaConsumeTopic  string `env:"KAFKA_CONSUME_TOPIC, default=process-image"`
	KafkaConsumerGroup string `env:"KAFKA_CONSUMER_GROUP, default=cg-compressor"`

	RedisAddress  string `env:"REDIS_ADDRESS, default=localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD, default=secret"`
	RedisDB       int    `env:"REDIS_DB, default=0"`

	AppName        string `env:"APP_NAME, default=image-compressor"`
	AppEnvironment string `env:"ENVIRONMENT, default=development"`
}

func LoadCfg(ctx context.Context) (Config, error) {
	var input In

	if err := envconfig.Process(ctx, &input); err != nil {
		return Config{}, err
	}

	cfg := Config{
		PubsubCfg: PubsubCfg{
			KafkaAddress:       input.KafkaAddress,
			KafkaConsumeTopic:  input.KafkaConsumeTopic,
			KafkaConsumerGroup: input.KafkaConsumerGroup,
		},
		CacheCfg: CacheCfg{
			RedisAddress:  input.RedisAddress,
			RedisPassword: input.RedisPassword,
			RedisDB:       input.RedisDB,
		},
		AppCfg: AppCfg{
			Name:        input.AppName,
			Environment: input.AppEnvironment,
		},
	}

	return cfg, nil
}
