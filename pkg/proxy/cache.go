package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/ionextai/git-scrapper/pkg/errorshelper"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

func GetProxyTarget(ctx context.Context, client *redis.Client, key string) (*httputil.ReverseProxy, error) {
	val, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	var cache ProxyCache
	if err := json.Unmarshal([]byte(val), &cache); err != nil {
		return nil, err
	}

	url, err := url.Parse(cache.URL)
	if err != nil {
		return nil, err
	}

	return NewProxy(url, cache.Token), nil
}

func CacheProxy(ctx context.Context, client *redis.Client, key string, cache ProxyCache, ttl time.Duration) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return client.Set(ctx, key, data, ttl).Err()
}

func InvalidateCache(ctx context.Context, client *redis.Client, key string) error {
	return client.Del(ctx, key).Err()
}

// getTokenTTL decodes the JWT and returns the TTL based on the expiration time.
func GetTokenTTL(tokenString string) (time.Duration, error) {
	// Parse JWT without verifying signature (since we only need the exp claim)
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid JWT claims")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return 0, fmt.Errorf("exp claim missing or invalid")
	}

	expirationTime := time.Unix(int64(exp), 0)
	ttl := time.Until(expirationTime)
	if ttl < 0 {
		return 0, errorshelper.ExpiredToken{}
	}

	return ttl, nil
}
