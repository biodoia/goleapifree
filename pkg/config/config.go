package config

import (
	"fmt"
	"os"

	"github.com/biodoia/goleapifree/pkg/database"
	"github.com/spf13/viper"
)

// Config rappresenta la configurazione completa dell'applicazione
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   database.Config  `yaml:"database"`
	Redis      RedisConfig      `yaml:"redis"`
	Providers  ProvidersConfig  `yaml:"providers"`
	Routing    RoutingConfig    `yaml:"routing"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

// ServerConfig configurazione del server
type ServerConfig struct {
	Port  int    `yaml:"port"`
	Host  string `yaml:"host"`
	HTTP3 bool   `yaml:"http3"`
	TLS   struct {
		Enabled bool   `yaml:"enabled"`
		Cert    string `yaml:"cert"`
		Key     string `yaml:"key"`
	} `yaml:"tls"`
}

// RedisConfig configurazione Redis
type RedisConfig struct {
	Host     string `yaml:"host"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// ProvidersConfig configurazione providers
type ProvidersConfig struct {
	AutoDiscovery        bool   `yaml:"auto_discovery"`
	HealthCheckInterval  string `yaml:"health_check_interval"`
	DefaultTimeout       string `yaml:"default_timeout"`
}

// RoutingConfig configurazione routing
type RoutingConfig struct {
	Strategy        string `yaml:"strategy"` // "cost_optimized", "latency_first", "quality_first"
	FailoverEnabled bool   `yaml:"failover_enabled"`
	MaxRetries      int    `yaml:"max_retries"`
}

// MonitoringConfig configurazione monitoring
type MonitoringConfig struct {
	Prometheus struct {
		Enabled bool `yaml:"enabled"`
		Port    int  `yaml:"port"`
	} `yaml:"prometheus"`
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
}

// Load carica la configurazione da file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// Read environment variables
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found, use defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults imposta i valori di default
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.http3", true)
	v.SetDefault("server.tls.enabled", false)

	// Database defaults
	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.connection", "./data/goleapai.db")
	v.SetDefault("database.max_conns", 25)
	v.SetDefault("database.log_level", "warn")

	// Redis defaults
	v.SetDefault("redis.host", "localhost:6379")
	v.SetDefault("redis.db", 0)

	// Providers defaults
	v.SetDefault("providers.auto_discovery", true)
	v.SetDefault("providers.health_check_interval", "5m")
	v.SetDefault("providers.default_timeout", "30s")

	// Routing defaults
	v.SetDefault("routing.strategy", "cost_optimized")
	v.SetDefault("routing.failover_enabled", true)
	v.SetDefault("routing.max_retries", 3)

	// Monitoring defaults
	v.SetDefault("monitoring.prometheus.enabled", true)
	v.SetDefault("monitoring.prometheus.port", 9090)
	v.SetDefault("monitoring.logging.level", "info")
	v.SetDefault("monitoring.logging.format", "json")
}

// Validate valida la configurazione
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Server.TLS.Enabled {
		if _, err := os.Stat(c.Server.TLS.Cert); os.IsNotExist(err) {
			return fmt.Errorf("TLS certificate not found: %s", c.Server.TLS.Cert)
		}
		if _, err := os.Stat(c.Server.TLS.Key); os.IsNotExist(err) {
			return fmt.Errorf("TLS key not found: %s", c.Server.TLS.Key)
		}
	}

	return nil
}
