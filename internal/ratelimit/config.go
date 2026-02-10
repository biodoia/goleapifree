package ratelimit

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// PresetConfig represents a preset configuration
type PresetConfig struct {
	Name        string
	Description string
	Configs     map[LimitLevel]Config
}

// Preset configurations for common use cases
var (
	// PresetFree is for free tier users
	PresetFree = PresetConfig{
		Name:        "free",
		Description: "Free tier rate limits",
		Configs: map[LimitLevel]Config{
			LimitLevelGlobal: {
				Level:     LimitLevelGlobal,
				Algorithm: AlgorithmTokenBucket,
				Limit:     1000,
				Window:    time.Hour,
				Burst:     100,
			},
			LimitLevelUser: {
				Level:     LimitLevelUser,
				Algorithm: AlgorithmSlidingWindowCounter,
				Limit:     60,
				Window:    time.Minute,
				Burst:     10,
			},
			LimitLevelProvider: {
				Level:     LimitLevelProvider,
				Algorithm: AlgorithmFixedWindow,
				Limit:     100,
				Window:    time.Minute,
				Burst:     10,
			},
		},
	}

	// PresetBasic is for basic paid users
	PresetBasic = PresetConfig{
		Name:        "basic",
		Description: "Basic tier rate limits",
		Configs: map[LimitLevel]Config{
			LimitLevelGlobal: {
				Level:     LimitLevelGlobal,
				Algorithm: AlgorithmTokenBucket,
				Limit:     5000,
				Window:    time.Hour,
				Burst:     500,
			},
			LimitLevelUser: {
				Level:     LimitLevelUser,
				Algorithm: AlgorithmSlidingWindowCounter,
				Limit:     200,
				Window:    time.Minute,
				Burst:     50,
			},
			LimitLevelProvider: {
				Level:     LimitLevelProvider,
				Algorithm: AlgorithmFixedWindow,
				Limit:     500,
				Window:    time.Minute,
				Burst:     50,
			},
		},
	}

	// PresetPro is for professional users
	PresetPro = PresetConfig{
		Name:        "pro",
		Description: "Professional tier rate limits",
		Configs: map[LimitLevel]Config{
			LimitLevelGlobal: {
				Level:     LimitLevelGlobal,
				Algorithm: AlgorithmTokenBucket,
				Limit:     20000,
				Window:    time.Hour,
				Burst:     2000,
			},
			LimitLevelUser: {
				Level:     LimitLevelUser,
				Algorithm: AlgorithmSlidingWindowCounter,
				Limit:     1000,
				Window:    time.Minute,
				Burst:     200,
			},
			LimitLevelProvider: {
				Level:     LimitLevelProvider,
				Algorithm: AlgorithmFixedWindow,
				Limit:     2000,
				Window:    time.Minute,
				Burst:     200,
			},
		},
	}

	// PresetEnterprise is for enterprise users
	PresetEnterprise = PresetConfig{
		Name:        "enterprise",
		Description: "Enterprise tier rate limits",
		Configs: map[LimitLevel]Config{
			LimitLevelGlobal: {
				Level:     LimitLevelGlobal,
				Algorithm: AlgorithmTokenBucket,
				Limit:     100000,
				Window:    time.Hour,
				Burst:     10000,
			},
			LimitLevelUser: {
				Level:     LimitLevelUser,
				Algorithm: AlgorithmSlidingWindowCounter,
				Limit:     10000,
				Window:    time.Minute,
				Burst:     1000,
			},
			LimitLevelProvider: {
				Level:     LimitLevelProvider,
				Algorithm: AlgorithmFixedWindow,
				Limit:     20000,
				Window:    time.Minute,
				Burst:     2000,
			},
		},
	}
)

// QuotaPresetConfig represents preset quota configurations
type QuotaPresetConfig struct {
	Name        string
	Description string
	Configs     map[string]QuotaConfig
}

// Quota presets
var (
	// QuotaPresetFree is for free tier
	QuotaPresetFree = QuotaPresetConfig{
		Name:        "free",
		Description: "Free tier quotas",
		Configs: map[string]QuotaConfig{
			"daily": {
				Limit:            1000,
				Period:           QuotaPeriodDaily,
				Type:             QuotaTypeHard,
				WarningThreshold: 0.8,
			},
			"monthly": {
				Limit:            10000,
				Period:           QuotaPeriodMonthly,
				Type:             QuotaTypeHard,
				WarningThreshold: 0.8,
			},
		},
	}

	// QuotaPresetBasic is for basic tier
	QuotaPresetBasic = QuotaPresetConfig{
		Name:        "basic",
		Description: "Basic tier quotas",
		Configs: map[string]QuotaConfig{
			"daily": {
				Limit:            10000,
				Period:           QuotaPeriodDaily,
				Type:             QuotaTypeHard,
				WarningThreshold: 0.8,
			},
			"monthly": {
				Limit:            200000,
				Period:           QuotaPeriodMonthly,
				Type:             QuotaTypeHard,
				WarningThreshold: 0.8,
			},
		},
	}

	// QuotaPresetPro is for pro tier
	QuotaPresetPro = QuotaPresetConfig{
		Name:        "pro",
		Description: "Professional tier quotas",
		Configs: map[string]QuotaConfig{
			"daily": {
				Limit:            100000,
				Period:           QuotaPeriodDaily,
				Type:             QuotaTypeSoft,
				WarningThreshold: 0.9,
			},
			"monthly": {
				Limit:            2000000,
				Period:           QuotaPeriodMonthly,
				Type:             QuotaTypeHard,
				WarningThreshold: 0.9,
			},
		},
	}

	// QuotaPresetEnterprise is for enterprise tier
	QuotaPresetEnterprise = QuotaPresetConfig{
		Name:        "enterprise",
		Description: "Enterprise tier quotas",
		Configs: map[string]QuotaConfig{
			"daily": {
				Limit:            1000000,
				Period:           QuotaPeriodDaily,
				Type:             QuotaTypeSoft,
				WarningThreshold: 0.95,
			},
			"monthly": {
				Limit:            20000000,
				Period:           QuotaPeriodMonthly,
				Type:             QuotaTypeSoft,
				WarningThreshold: 0.95,
			},
		},
	}
)

// Builder helps build a configured multi-level limiter
type Builder struct {
	limiter      *MultiLevelLimiter
	redisClient  *redis.Client
	distributed  bool
	keyPrefix    string
	premiumUsers []string
	whitelisted  []string
}

// NewBuilder creates a new builder
func NewBuilder() *Builder {
	return &Builder{
		limiter:   NewMultiLevelLimiter(),
		keyPrefix: "ratelimit",
	}
}

// WithRedis enables distributed rate limiting
func (b *Builder) WithRedis(client *redis.Client) *Builder {
	b.redisClient = client
	b.distributed = true
	return b
}

// WithKeyPrefix sets the Redis key prefix
func (b *Builder) WithKeyPrefix(prefix string) *Builder {
	b.keyPrefix = prefix
	return b
}

// WithPreset applies a preset configuration
func (b *Builder) WithPreset(preset PresetConfig) *Builder {
	for level, config := range preset.Configs {
		config.KeyPrefix = b.keyPrefix
		config.Distributed = b.distributed

		var limiter Limiter
		var err error

		if b.distributed && b.redisClient != nil {
			limiter = NewDistributedLimiter(config, b.redisClient)
		} else {
			limiter, err = NewLimiter(config)
			if err != nil {
				// Log error or handle appropriately
				continue
			}
		}

		b.limiter.AddLimiter(level, limiter)
	}

	return b
}

// WithCustomLimit adds a custom limit for a level
func (b *Builder) WithCustomLimit(level LimitLevel, config Config) *Builder {
	config.KeyPrefix = b.keyPrefix
	config.Distributed = b.distributed

	var limiter Limiter
	var err error

	if b.distributed && b.redisClient != nil {
		limiter = NewDistributedLimiter(config, b.redisClient)
	} else {
		limiter, err = NewLimiter(config)
		if err != nil {
			return b
		}
	}

	b.limiter.AddLimiter(level, limiter)
	return b
}

// WithPremiumUsers adds premium users
func (b *Builder) WithPremiumUsers(userIDs ...string) *Builder {
	b.premiumUsers = append(b.premiumUsers, userIDs...)
	return b
}

// WithWhitelisted adds whitelisted identifiers
func (b *Builder) WithWhitelisted(identifiers ...string) *Builder {
	b.whitelisted = append(b.whitelisted, identifiers...)
	return b
}

// Build returns the configured limiter
func (b *Builder) Build() *MultiLevelLimiter {
	// Add premium users
	for _, userID := range b.premiumUsers {
		b.limiter.AddPremiumUser(userID)
	}

	// Add whitelisted
	for _, identifier := range b.whitelisted {
		b.limiter.AddWhitelisted(identifier)
	}

	return b.limiter
}

// QuotaBuilder helps build quota managers
type QuotaBuilder struct {
	manager      *MultiQuotaManager
	redisClient  *redis.Client
	distributed  bool
	keyPrefix    string
}

// NewQuotaBuilder creates a new quota builder
func NewQuotaBuilder() *QuotaBuilder {
	return &QuotaBuilder{
		manager:   NewMultiQuotaManager(),
		keyPrefix: "quota",
	}
}

// WithRedis enables distributed quota management
func (qb *QuotaBuilder) WithRedis(client *redis.Client) *QuotaBuilder {
	qb.redisClient = client
	qb.distributed = true
	return qb
}

// WithKeyPrefix sets the Redis key prefix
func (qb *QuotaBuilder) WithKeyPrefix(prefix string) *QuotaBuilder {
	qb.keyPrefix = prefix
	return qb
}

// WithPreset applies a preset quota configuration
func (qb *QuotaBuilder) WithPreset(preset QuotaPresetConfig) *QuotaBuilder {
	for name, config := range preset.Configs {
		config.KeyPrefix = qb.keyPrefix
		config.Distributed = qb.distributed

		var manager QuotaManager
		if qb.distributed && qb.redisClient != nil {
			manager = NewDistributedQuotaManager(config, qb.redisClient)
		} else {
			manager = NewLocalQuotaManager(config)
		}

		qb.manager.AddQuota(name, manager)
	}

	return qb
}

// WithCustomQuota adds a custom quota
func (qb *QuotaBuilder) WithCustomQuota(name string, config QuotaConfig) *QuotaBuilder {
	config.KeyPrefix = qb.keyPrefix
	config.Distributed = qb.distributed

	var manager QuotaManager
	if qb.distributed && qb.redisClient != nil {
		manager = NewDistributedQuotaManager(config, qb.redisClient)
	} else {
		manager = NewLocalQuotaManager(config)
	}

	qb.manager.AddQuota(name, manager)
	return qb
}

// Build returns the configured quota manager
func (qb *QuotaBuilder) Build() *MultiQuotaManager {
	return qb.manager
}

// GetPresetByName returns a preset configuration by name
func GetPresetByName(name string) (*PresetConfig, error) {
	presets := map[string]PresetConfig{
		"free":       PresetFree,
		"basic":      PresetBasic,
		"pro":        PresetPro,
		"enterprise": PresetEnterprise,
	}

	preset, ok := presets[name]
	if !ok {
		return nil, fmt.Errorf("preset %s not found", name)
	}

	return &preset, nil
}

// GetQuotaPresetByName returns a quota preset by name
func GetQuotaPresetByName(name string) (*QuotaPresetConfig, error) {
	presets := map[string]QuotaPresetConfig{
		"free":       QuotaPresetFree,
		"basic":      QuotaPresetBasic,
		"pro":        QuotaPresetPro,
		"enterprise": QuotaPresetEnterprise,
	}

	preset, ok := presets[name]
	if !ok {
		return nil, fmt.Errorf("quota preset %s not found", name)
	}

	return &preset, nil
}
