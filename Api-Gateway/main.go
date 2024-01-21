package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
	"context"

	"github.com/gorilla/mux"
	"github.com/go-redis/redis/v8"

)

// RoundRobinLoadBalancer is a simple round-robin load balancer.
type RoundRobinLoadBalancer struct {
	mu      sync.Mutex
	servers []*url.URL
	index   int
}

// NextServer returns the next server in a round-robin fashion.
func (lb *RoundRobinLoadBalancer) NextServer() *url.URL {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.servers) == 0 {
		return nil
	}

	server := lb.servers[lb.index]
	lb.index = (lb.index + 1) % len(lb.servers)
	return server
}

// NewRoundRobinLoadBalancer creates a new RoundRobinLoadBalancer with the given servers.
func NewRoundRobinLoadBalancer(servers []string) *RoundRobinLoadBalancer {
	var urls []*url.URL
	for _, s := range servers {
		u, err := url.Parse(s)
		if err != nil {
			log.Fatal(err)
		}
		urls = append(urls, u)
	}

	return &RoundRobinLoadBalancer{servers: urls}
}

// RedisCache provides an interface for caching to Redis.
type RedisCache struct {
	client *redis.Client
}

// CacheKey returns a cache key for the given URL.
func CacheKey(u *url.URL) string {
	return u.String()
}

// SetCache sets the value in the cache with the specified key and value.
func (rc *RedisCache) SetCache(key string, value string, expiration time.Duration) error {
	return rc.client.Set(context.Background(), key, value, expiration).Err()
}

// GetCache gets the value from the cache for the specified key.
func (rc *RedisCache) GetCache(key string) (string, error) {
	return rc.client.Get(context.Background(), key).Result()
}

func main() {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "redis-container.pad_reexam:6379", // Update with your Redis server address
		DB:   0,
	})

	// Create a RedisCache instance
	redisCache := &RedisCache{client: redisClient}

	// Define the sports and imgur service endpoints
	sportsService := NewRoundRobinLoadBalancer([]string{
		"http://sports1-container.pad_reexam:5000",
		"http://sports2-container.pad_reexam:5000",
	})

	imgurService := NewRoundRobinLoadBalancer([]string{
		"http://imgur1-container.pad_reexam:5000",
		"http://imgur2-container.pad_reexam:5000",
	})

	// Create a new router using gorilla/mux
	router := mux.NewRouter()

	// Define handlers for sports service
	router.HandleFunc("/", createReverseProxyHandler(sportsService))
	router.HandleFunc("/_getsportlist", createReverseProxyHandler(sportsService))

	// Define handlers for imgur service
	router.HandleFunc("/search", createCacheHandler(redisCache, createReverseProxyHandler(imgurService)))

	// Start the API gateway on port 8080
	fmt.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

// createReverseProxyHandler creates a reverse proxy handler for the given load balancer.
func createReverseProxyHandler(lb *RoundRobinLoadBalancer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the next server from the load balancer
		target := lb.NextServer()

		// Create a reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(target)

		// Update the request URL to the target URL
		r.URL.Host = target.Host
		r.URL.Scheme = target.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		r.Host = target.Host

		// Serve the request using the reverse proxy
		proxy.ServeHTTP(w, r)
	}
}

// createCacheHandler creates a handler that caches the result of the request to Redis.
func createCacheHandler(cache *RedisCache, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is a "/search" endpoint
		if r.URL.Path == "/search" {
			// Generate a cache key based on the request URL
			cacheKey := CacheKey(r.URL)

			// Try to get the result from the cache
			cachedResult, err := cache.GetCache(cacheKey)
			if err == nil {
				// If the result is in the cache, serve it
				fmt.Println("Cache hit for key:", cacheKey)
				w.Write([]byte(cachedResult))
				return
			}

			// If the result is not in the cache, proceed with the request
			fmt.Println("Cache miss for key:", cacheKey)

			// Capture the response to get the result
			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, r)

			// Set the result in the cache with a specified expiration time
			cache.SetCache(cacheKey, recorder.Body.String(), time.Minute)

			// Copy the recorded response to the actual response writer
			recorder.Result().Write(w)
		} else {
			// For non-cached endpoints, simply proceed with the next handler
			next.ServeHTTP(w, r)
		}
	}
}
