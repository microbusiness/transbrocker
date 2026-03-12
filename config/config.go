package config

import (
	"github.com/rs/zerolog/log"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	NatsUrl     string `env:"NATS_URL,required"`
	NatsSubject string `env:"NATS_SUBJECT,required"`
	KafkaUrl    string `env:"KAFKA_URL,required"`
	KafkaTopic  string `env:"KAFKA_TOPIC,required"`
	HttpAddr    string `env:"HTTP_ADDR,required"`
	HttpPort    string `env:"HTTP_PORT,required"`

	CacheMaxCount   *int `env:"CACHE_MAX_COUNT" envDefault:"100"`
	CleanupInterval *int `env:"CLEANUP_INTERVAL" envDefault:"120"`
	DisableCache    bool `env:"DISABLE_CACHE" envDefault:"false"`
}

func New() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Warn().Msg("env variables NOT loaded from .env file, using environment")
	}
	cfg := new(Config)
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
