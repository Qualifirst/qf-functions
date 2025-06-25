package app

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
)

type contextKey string

const cacheContextKey = contextKey("app-cache")

type Cache struct {
	items map[string]any
	mu    sync.RWMutex
}

func cacheKey(args ...any) string {
	largs := make([]string, len(args))
	for i, a := range args {
		largs[i] = fmt.Sprintf("%v", a)
	}
	cacheKey := strings.Join(largs, "/")
	return cacheKey
}

func GetCacheValue[T any](ctx context.Context, key []any, fallback T) (val T, found bool) {
	cache := ctx.Value(cacheContextKey).(*Cache)
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	res, inCache := cache.items[cacheKey(key...)]
	if inCache {
		val, assertOk := res.(T)
		if assertOk {
			return val, true
		}
	}
	return fallback, false
}

func SetCacheValue(ctx context.Context, key []any, val any) func() {
	cache := ctx.Value(cacheContextKey).(*Cache)
	cache.mu.Lock()

	ck := cacheKey(key...)
	original, originalFound := cache.items[ck]
	cache.items[ck] = val

	cache.mu.Unlock()

	return func() {
		cache.mu.Lock()

		if originalFound {
			cache.items[ck] = original
		} else {
			delete(cache.items, ck)
		}

		cache.mu.Unlock()
	}
}

func ContextWithCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheContextKey, &Cache{
		items: map[string]any{},
	})
}

func CacheMiddleware(function NetlifyFunction) NetlifyFunction {
	return func(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
		return function(ContextWithCache(ctx), request)
	}
}
