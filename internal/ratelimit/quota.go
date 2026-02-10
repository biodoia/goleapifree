package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// QuotaPeriod represents the period for quota reset
type QuotaPeriod string

const (
	QuotaPeriodHourly  QuotaPeriod = "hourly"
	QuotaPeriodDaily   QuotaPeriod = "daily"
	QuotaPeriodWeekly  QuotaPeriod = "weekly"
	QuotaPeriodMonthly QuotaPeriod = "monthly"
)

// QuotaType represents the type of quota limit
type QuotaType string

const (
	QuotaTypeSoft QuotaType = "soft" // Warning, but allows requests
	QuotaTypeHard QuotaType = "hard" // Blocks requests
)

// QuotaConfig represents quota configuration
type QuotaConfig struct {
	// Quota limit
	Limit int64

	// Period for quota reset
	Period QuotaPeriod

	// Type (soft or hard)
	Type QuotaType

	// Warning threshold (percentage, e.g., 0.8 for 80%)
	WarningThreshold float64

	// Distributed mode (using Redis)
	Distributed bool

	// Key prefix for Redis
	KeyPrefix string
}

// QuotaInfo contains information about quota usage
type QuotaInfo struct {
	// Allowed indicates if the request is within quota
	Allowed bool

	// Limit is the quota limit
	Limit int64

	// Used is the amount of quota used
	Used int64

	// Remaining is the remaining quota
	Remaining int64

	// Reset is when the quota will reset
	Reset time.Time

	// Warning indicates if usage is above warning threshold
	Warning bool

	// Type is the quota type
	Type QuotaType
}

// QuotaManager manages quota limits
type QuotaManager interface {
	// Use consumes quota and returns quota info
	Use(ctx context.Context, key string, amount int64) (*QuotaInfo, error)

	// GetInfo returns current quota info without consuming
	GetInfo(ctx context.Context, key string) (*QuotaInfo, error)

	// Reset resets quota for a key
	Reset(ctx context.Context, key string) error

	// SetLimit updates the quota limit for a key
	SetLimit(ctx context.Context, key string, limit int64) error
}

// LocalQuotaManager implements quota management in memory
type LocalQuotaManager struct {
	config QuotaConfig
	quotas sync.Map // map[string]*quotaEntry
	mu     sync.RWMutex
}

type quotaEntry struct {
	used        int64
	limit       int64
	periodStart time.Time
	mu          sync.Mutex
}

// NewLocalQuotaManager creates a new local quota manager
func NewLocalQuotaManager(config QuotaConfig) *LocalQuotaManager {
	return &LocalQuotaManager{
		config: config,
	}
}

// Use consumes quota
func (q *LocalQuotaManager) Use(ctx context.Context, key string, amount int64) (*QuotaInfo, error) {
	now := time.Now()
	periodStart := q.getPeriodStart(now)

	entryInterface, _ := q.quotas.LoadOrStore(key, &quotaEntry{
		used:        0,
		limit:       q.config.Limit,
		periodStart: periodStart,
	})

	entry := entryInterface.(*quotaEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Reset if new period
	if entry.periodStart.Before(periodStart) {
		entry.used = 0
		entry.periodStart = periodStart
	}

	// Check quota
	newUsed := entry.used + amount
	allowed := true

	if q.config.Type == QuotaTypeHard {
		allowed = newUsed <= entry.limit
	}

	// Update usage if allowed or soft limit
	if allowed || q.config.Type == QuotaTypeSoft {
		entry.used = newUsed
	}

	remaining := entry.limit - entry.used
	if remaining < 0 {
		remaining = 0
	}

	// Check warning threshold
	warning := false
	if q.config.WarningThreshold > 0 {
		usagePercent := float64(entry.used) / float64(entry.limit)
		warning = usagePercent >= q.config.WarningThreshold
	}

	reset := q.getNextReset(periodStart)

	return &QuotaInfo{
		Allowed:   allowed,
		Limit:     entry.limit,
		Used:      entry.used,
		Remaining: remaining,
		Reset:     reset,
		Warning:   warning,
		Type:      q.config.Type,
	}, nil
}

// GetInfo returns current quota info
func (q *LocalQuotaManager) GetInfo(ctx context.Context, key string) (*QuotaInfo, error) {
	now := time.Now()
	periodStart := q.getPeriodStart(now)

	entryInterface, ok := q.quotas.Load(key)
	if !ok {
		return &QuotaInfo{
			Allowed:   true,
			Limit:     q.config.Limit,
			Used:      0,
			Remaining: q.config.Limit,
			Reset:     q.getNextReset(periodStart),
			Warning:   false,
			Type:      q.config.Type,
		}, nil
	}

	entry := entryInterface.(*quotaEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	used := entry.used
	limit := entry.limit

	// Reset if new period
	if entry.periodStart.Before(periodStart) {
		used = 0
	}

	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	warning := false
	if q.config.WarningThreshold > 0 {
		usagePercent := float64(used) / float64(limit)
		warning = usagePercent >= q.config.WarningThreshold
	}

	return &QuotaInfo{
		Allowed:   remaining > 0 || q.config.Type == QuotaTypeSoft,
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
		Reset:     q.getNextReset(periodStart),
		Warning:   warning,
		Type:      q.config.Type,
	}, nil
}

// Reset resets quota for a key
func (q *LocalQuotaManager) Reset(ctx context.Context, key string) error {
	q.quotas.Delete(key)
	return nil
}

// SetLimit updates the quota limit for a key
func (q *LocalQuotaManager) SetLimit(ctx context.Context, key string, limit int64) error {
	entryInterface, ok := q.quotas.Load(key)
	if !ok {
		now := time.Now()
		q.quotas.Store(key, &quotaEntry{
			used:        0,
			limit:       limit,
			periodStart: q.getPeriodStart(now),
		})
		return nil
	}

	entry := entryInterface.(*quotaEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.limit = limit
	return nil
}

// getPeriodStart returns the start time of the current period
func (q *LocalQuotaManager) getPeriodStart(now time.Time) time.Time {
	switch q.config.Period {
	case QuotaPeriodHourly:
		return now.Truncate(time.Hour)
	case QuotaPeriodDaily:
		year, month, day := now.Date()
		return time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	case QuotaPeriodWeekly:
		year, month, day := now.Date()
		weekday := now.Weekday()
		// Go back to Sunday
		daysBack := int(weekday)
		startDate := time.Date(year, month, day, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -daysBack)
		return startDate
	case QuotaPeriodMonthly:
		year, month, _ := now.Date()
		return time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	default:
		return now.Truncate(24 * time.Hour)
	}
}

// getNextReset returns when the quota will reset
func (q *LocalQuotaManager) getNextReset(periodStart time.Time) time.Time {
	switch q.config.Period {
	case QuotaPeriodHourly:
		return periodStart.Add(time.Hour)
	case QuotaPeriodDaily:
		return periodStart.AddDate(0, 0, 1)
	case QuotaPeriodWeekly:
		return periodStart.AddDate(0, 0, 7)
	case QuotaPeriodMonthly:
		return periodStart.AddDate(0, 1, 0)
	default:
		return periodStart.Add(24 * time.Hour)
	}
}

// DistributedQuotaManager implements quota management using Redis
type DistributedQuotaManager struct {
	config QuotaConfig
	redis  *redis.Client
}

// NewDistributedQuotaManager creates a new distributed quota manager
func NewDistributedQuotaManager(config QuotaConfig, redisClient *redis.Client) *DistributedQuotaManager {
	return &DistributedQuotaManager{
		config: config,
		redis:  redisClient,
	}
}

// Use consumes quota
func (d *DistributedQuotaManager) Use(ctx context.Context, key string, amount int64) (*QuotaInfo, error) {
	now := time.Now()
	periodStart := d.getPeriodStart(now)
	reset := d.getNextReset(periodStart)

	redisKey := d.getRedisKey(key)
	limitKey := redisKey + ":limit"

	// Lua script for atomic quota operations
	script := redis.NewScript(`
		local usage_key = KEYS[1]
		local limit_key = KEYS[2]
		local amount = tonumber(ARGV[1])
		local default_limit = tonumber(ARGV[2])
		local is_hard = ARGV[3] == "1"
		local ttl = tonumber(ARGV[4])

		-- Get current usage and limit
		local used = tonumber(redis.call('GET', usage_key)) or 0
		local limit = tonumber(redis.call('GET', limit_key)) or default_limit

		-- Check and update
		local new_used = used + amount
		local allowed = 1

		if is_hard and new_used > limit then
			allowed = 0
		else
			redis.call('INCRBY', usage_key, amount)
			redis.call('EXPIRE', usage_key, ttl)
			used = new_used
		end

		return {allowed, used, limit}
	`)

	isHard := "0"
	if d.config.Type == QuotaTypeHard {
		isHard = "1"
	}

	ttl := int(time.Until(reset).Seconds()) + 60

	result, err := script.Run(ctx, d.redis, []string{redisKey, limitKey},
		amount,
		d.config.Limit,
		isHard,
		ttl,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("quota script failed: %w", err)
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1
	used := resultSlice[1].(int64)
	limit := resultSlice[2].(int64)

	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	warning := false
	if d.config.WarningThreshold > 0 {
		usagePercent := float64(used) / float64(limit)
		warning = usagePercent >= d.config.WarningThreshold
	}

	return &QuotaInfo{
		Allowed:   allowed,
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
		Reset:     reset,
		Warning:   warning,
		Type:      d.config.Type,
	}, nil
}

// GetInfo returns current quota info
func (d *DistributedQuotaManager) GetInfo(ctx context.Context, key string) (*QuotaInfo, error) {
	now := time.Now()
	periodStart := d.getPeriodStart(now)
	reset := d.getNextReset(periodStart)

	redisKey := d.getRedisKey(key)
	limitKey := redisKey + ":limit"

	pipe := d.redis.Pipeline()
	usedCmd := pipe.Get(ctx, redisKey)
	limitCmd := pipe.Get(ctx, limitKey)
	_, _ = pipe.Exec(ctx)

	used := int64(0)
	limit := d.config.Limit

	if val, err := usedCmd.Result(); err == nil {
		used, _ = strconv.ParseInt(val, 10, 64)
	}

	if val, err := limitCmd.Result(); err == nil {
		limit, _ = strconv.ParseInt(val, 10, 64)
	}

	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	warning := false
	if d.config.WarningThreshold > 0 {
		usagePercent := float64(used) / float64(limit)
		warning = usagePercent >= d.config.WarningThreshold
	}

	return &QuotaInfo{
		Allowed:   remaining > 0 || d.config.Type == QuotaTypeSoft,
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
		Reset:     reset,
		Warning:   warning,
		Type:      d.config.Type,
	}, nil
}

// Reset resets quota for a key
func (d *DistributedQuotaManager) Reset(ctx context.Context, key string) error {
	redisKey := d.getRedisKey(key)
	return d.redis.Del(ctx, redisKey).Err()
}

// SetLimit updates the quota limit for a key
func (d *DistributedQuotaManager) SetLimit(ctx context.Context, key string, limit int64) error {
	limitKey := d.getRedisKey(key) + ":limit"

	now := time.Now()
	periodStart := d.getPeriodStart(now)
	reset := d.getNextReset(periodStart)
	ttl := time.Until(reset) + time.Hour

	return d.redis.Set(ctx, limitKey, limit, ttl).Err()
}

// getPeriodStart returns the start time of the current period
func (d *DistributedQuotaManager) getPeriodStart(now time.Time) time.Time {
	switch d.config.Period {
	case QuotaPeriodHourly:
		return now.Truncate(time.Hour)
	case QuotaPeriodDaily:
		year, month, day := now.Date()
		return time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	case QuotaPeriodWeekly:
		year, month, day := now.Date()
		weekday := now.Weekday()
		daysBack := int(weekday)
		startDate := time.Date(year, month, day, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -daysBack)
		return startDate
	case QuotaPeriodMonthly:
		year, month, _ := now.Date()
		return time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	default:
		return now.Truncate(24 * time.Hour)
	}
}

// getNextReset returns when the quota will reset
func (d *DistributedQuotaManager) getNextReset(periodStart time.Time) time.Time {
	switch d.config.Period {
	case QuotaPeriodHourly:
		return periodStart.Add(time.Hour)
	case QuotaPeriodDaily:
		return periodStart.AddDate(0, 0, 1)
	case QuotaPeriodWeekly:
		return periodStart.AddDate(0, 0, 7)
	case QuotaPeriodMonthly:
		return periodStart.AddDate(0, 1, 0)
	default:
		return periodStart.Add(24 * time.Hour)
	}
}

// getRedisKey generates a Redis key for the quota
func (d *DistributedQuotaManager) getRedisKey(key string) string {
	prefix := d.config.KeyPrefix
	if prefix == "" {
		prefix = "quota"
	}

	now := time.Now()
	periodStart := d.getPeriodStart(now)

	// Include period in key to auto-reset
	periodKey := periodStart.Format("2006-01-02-15")
	return fmt.Sprintf("%s:%s:%s", prefix, key, periodKey)
}

// MultiQuotaManager manages multiple quota types
type MultiQuotaManager struct {
	quotas map[string]QuotaManager
	mu     sync.RWMutex
}

// NewMultiQuotaManager creates a new multi-quota manager
func NewMultiQuotaManager() *MultiQuotaManager {
	return &MultiQuotaManager{
		quotas: make(map[string]QuotaManager),
	}
}

// AddQuota adds a quota manager with a name
func (m *MultiQuotaManager) AddQuota(name string, manager QuotaManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.quotas[name] = manager
}

// Use consumes quota from all managed quotas
func (m *MultiQuotaManager) Use(ctx context.Context, key string, amount int64) (map[string]*QuotaInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*QuotaInfo)

	for name, manager := range m.quotas {
		info, err := manager.Use(ctx, key, amount)
		if err != nil {
			return nil, fmt.Errorf("using quota %s: %w", name, err)
		}

		result[name] = info

		// If any hard limit is exceeded, stop
		if !info.Allowed && info.Type == QuotaTypeHard {
			return result, nil
		}
	}

	return result, nil
}

// GetInfo returns info from all managed quotas
func (m *MultiQuotaManager) GetInfo(ctx context.Context, key string) (map[string]*QuotaInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*QuotaInfo)

	for name, manager := range m.quotas {
		info, err := manager.GetInfo(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("getting quota %s info: %w", name, err)
		}

		result[name] = info
	}

	return result, nil
}
