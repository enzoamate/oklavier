package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides a Redis-backed caching layer with graceful degradation.
// If Valkey/Redis is unavailable, all operations are no-ops and callers
// fall through to the database.
type Cache struct {
	client *redis.Client
}

// New creates a cache backed by Valkey/Redis. If valkeyURL is empty or the
// connection fails, caching is silently disabled.
func New(valkeyURL string) *Cache {
	if valkeyURL == "" {
		log.Println("Cache: no Valkey URL, caching disabled")
		return &Cache{}
	}
	client := redis.NewClient(&redis.Options{Addr: valkeyURL})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Cache: Valkey connection failed: %v, caching disabled", err)
		return &Cache{}
	}
	log.Println("Cache: connected to Valkey")
	return &Cache{client: client}
}

// Get retrieves a cached value by key and unmarshals it into dest.
// Returns true if the value was found and successfully unmarshalled.
func (c *Cache) Get(key string, dest interface{}) bool {
	if c.client == nil {
		return false
	}
	val, err := c.client.Get(context.Background(), "cache:"+key).Result()
	if err != nil {
		return false
	}
	return json.Unmarshal([]byte(val), dest) == nil
}

// Set stores a value in the cache with the given TTL.
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	if c.client == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	c.client.Set(context.Background(), "cache:"+key, data, ttl)
}

// Delete removes one or more specific keys from the cache.
func (c *Cache) Delete(keys ...string) {
	if c.client == nil {
		return
	}
	prefixed := make([]string, len(keys))
	for i, k := range keys {
		prefixed[i] = "cache:" + k
	}
	c.client.Del(context.Background(), prefixed...)
}

// Invalidate removes all cache keys matching the given prefix.
func (c *Cache) Invalidate(prefix string) {
	if c.client == nil {
		return
	}
	ctx := context.Background()
	iter := c.client.Scan(ctx, 0, "cache:"+prefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		c.client.Del(ctx, iter.Val())
	}
}
