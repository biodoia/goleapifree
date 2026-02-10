package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wrapper per redis client
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient crea un nuovo client Redis
func NewRedisClient(host, password string, db int) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         host,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

// Get ottiene un valore dalla cache
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Set imposta un valore nella cache
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Del elimina una chiave dalla cache
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Incr incrementa un contatore
func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// IncrBy incrementa un contatore di un valore specifico
func (r *RedisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, key, value).Result()
}

// Expire imposta una scadenza per una chiave
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// TTL ottiene il tempo rimanente di una chiave
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Exists verifica se una chiave esiste
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

// ZAdd aggiunge un elemento a un sorted set
func (r *RedisClient) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return r.client.ZAdd(ctx, key, members...).Err()
}

// ZRemRangeByScore rimuove elementi da un sorted set per score
func (r *RedisClient) ZRemRangeByScore(ctx context.Context, key, min, max string) error {
	return r.client.ZRemRangeByScore(ctx, key, min, max).Err()
}

// ZCount conta elementi in un sorted set per range di score
func (r *RedisClient) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return r.client.ZCount(ctx, key, min, max).Result()
}

// Pipeline crea una pipeline per comandi batch
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// Close chiude la connessione Redis
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Client restituisce il client Redis nativo
func (r *RedisClient) Client() *redis.Client {
	return r.client
}
