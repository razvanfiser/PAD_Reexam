package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/afex/hystrix-go/hystrix"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
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

func init() {
	// Shared Hystrix configuration for both services
	hystrix.ConfigureCommand("shared", hystrix.CommandConfig{
		Timeout:               5000, // in milliseconds
		MaxConcurrentRequests: 4,  // max number of concurrent requests
		ErrorPercentThreshold: 25,   // error rate threshold percentage
	})
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
		"http://sports3-container.pad_reexam:5000",
	})

	imgurService := NewRoundRobinLoadBalancer([]string{
		"http://imgur1-container.pad_reexam:5000",
		"http://imgur2-container.pad_reexam:5000",
		"http://imgur3-container.pad_reexam:5000",
	})

	// Create a new router using gorilla/mux
	router := mux.NewRouter()

	// Update the sports and imgur service endpoints with Hystrix-enabled handlers
	router.HandleFunc("/", createReverseProxyHandler(sportsService, "sports"))
	router.HandleFunc("/_getsportlist", createReverseProxyHandler(sportsService, "sports"))
	router.HandleFunc("/sports/{sport_id:[0-9]+}", createReverseProxyHandler(sportsService, "sports"))

	router.HandleFunc("/events/{sport_id:[0-9]+}/live", createReverseProxyHandler(sportsService, "sports"))
	router.HandleFunc("/events/date/{date}", createReverseProxyHandler(sportsService, "sports"))
	router.HandleFunc("/teams/{sport_id:[0-9]+}", createReverseProxyHandler(sportsService, "sports"))
	router.HandleFunc("/players/{sport_id:[0-9]+}", createReverseProxyHandler(sportsService, "sports"))
	router.HandleFunc("/sports/timeout", createReverseProxyHandler(sportsService, "sports"))

	router.HandleFunc("/search", createCacheHandler(redisCache, createReverseProxyHandler(imgurService, "imgur")))
	router.HandleFunc("/tag/{tag}", createReverseProxyHandler(imgurService, "imgur"))
	router.HandleFunc("/album", createReverseProxyHandler(imgurService, "imgur"))
	router.HandleFunc("/upload", createReverseProxyHandler(imgurService, "imgur"))
	router.HandleFunc("/imgur/timeout", createReverseProxyHandler(imgurService, "imgur"))

	router.HandleFunc("/images/players/{sport_id:[0-9]+}", createImagesPlayersHandler(sportsService, imgurService, redisCache))
	router.HandleFunc("/images/teams/{sport_id:[0-9]+}", createImagesPlayersHandler(sportsService, imgurService, redisCache))
	router.HandleFunc("/images/events/{sport_id:[0-9]+}/live", createImagesPlayersHandler(sportsService, imgurService, redisCache))

	router.HandleFunc("/status", createStatusHandler(sportsService, imgurService))


	// Start the API gateway on port 8080
	fmt.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func createStatusHandler(sportsLB *RoundRobinLoadBalancer, imgurLB *RoundRobinLoadBalancer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Make requests to the /status endpoints of imgur and sports services
		sportsStatus, err1 := makeProxyRequest(sportsLB, r, "/status")
		imgurStatus, err2 := makeProxyRequest(imgurLB, r, "/status")

		// Check for errors in making the requests
		//if err1 != nil || err2 != nil {
		//	// Handle errors (e.g., return an error response)
		//	w.WriteHeader(http.StatusInternalServerError)
		//	w.Write([]byte("Error fetching status from services"))
		//	return
		//}
		if err1 != nil {
			sportsStatus = "Server not responding"
		}
		if err2 != nil {
			imgurStatus = "Server not responding"
		}

		// Aggregate the results into a JSON-format response
		statusResponse := fmt.Sprintf("{\"sports\": %s, \"imgur\": %s}", sportsStatus, imgurStatus)

		// Set the content type and write the response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(statusResponse))
	}
}

// createReverseProxyHandler creates a reverse proxy handler for the given load balancer.
func createReverseProxyHandler(lb *RoundRobinLoadBalancer, serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Use Hystrix for circuit-breaking and timeout with the shared configuration
		err := hystrix.Do("shared", func() error {
			// Get the next server from the load balancer
			target := lb.NextServer()
			proxy := httputil.NewSingleHostReverseProxy(target)

			// Update the request URL to the target URL
			r.URL.Host = target.Host
			r.URL.Scheme = target.Scheme
			r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
			r.Host = target.Host

			// Serve the request using the reverse proxy
			proxy.ServeHTTP(w, r)

			return nil
		}, nil)

		// Check if the error is a CircuitError, which includes a timeout
		if err != nil && err == hystrix.ErrTimeout {
			// Set the HTTP status code to 408 (Request Timeout)
			w.WriteHeader(http.StatusRequestTimeout)
			w.Write([]byte("Request Timeout"))
			fmt.Println("Hystrix circuit opened for", serviceName, "with error:", err)
		} else if err != nil {
			// For other errors, set the HTTP status code to 503 (Service Unavailable)
			w.WriteHeader(http.StatusServiceUnavailable)
			// Log the error for further investigation
			fmt.Println("Hystrix circuit opened for", serviceName, "with error:", err)
		}
	}
}

var sportNameMapping = map[int]string{
	1: "football",
	2: "tennis",
	3: "basketball",
	4: "ice+hockey",
	5: "volleyball",
	6: "handball",
	// Add more mappings as needed
}

func createImagesPlayersHandler(sportsLB *RoundRobinLoadBalancer, imgurLB *RoundRobinLoadBalancer, cache *RedisCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := hystrix.Do("shared", func() error {
			initPath := r.URL.Path
			// Extract sport_id from the URL parameters
			vars := mux.Vars(r)
			sportID, err := strconv.Atoi(vars["sport_id"])
			if err != nil {
				// Handle error (e.g., invalid sport_id)
				http.Error(w, "Invalid sport_id", http.StatusBadRequest)
				//return
			}

			// Map sport_id to sport name
			sportName, ok := sportNameMapping[sportID]
			if !ok {
				// Handle error (e.g., sport_id not found in the mapping)
				http.Error(w, "Sport not found", http.StatusNotFound)
				//return
			}

			//sportPath := fmt.Sprintf("/players/%d", sportID)
			sportPath := strings.Split(initPath, "/images")[1]
			// Make a request to "/players/{sport_id:[0-9]+}" in the sports service
			playersURL := sportPath
			sportsResponse, err := makeProxyRequest(sportsLB, r, playersURL)
			if err != nil {
				// Handle error
				http.Error(w, "Error fetching players data", http.StatusInternalServerError)
				//return
			}

			query := url.Values{}
			query.Add("q", sportName)
			path := "/search"
			imgurURL := path + "?" + query.Encode()
			imgurResponse, err := makeProxyRequest(imgurLB, r, imgurURL)
			if err != nil {
				// Handle error
				http.Error(w, "Error fetching imgur data", http.StatusInternalServerError)
				//return
			}

			result := fmt.Sprintf("{\"sports\": %s, \"imgur\": %s}", sportsResponse, imgurResponse)

			// Set the content type and write the response
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result))

			return nil
		}, nil)

		// Check if the error is a CircuitError, which includes a timeout
		if err != nil && err == hystrix.ErrTimeout {
			// Set the HTTP status code to 408 (Request Timeout)
			w.WriteHeader(http.StatusRequestTimeout)
			fmt.Println("Hystrix circuit opened for API gateway with error:", err)
		} else if err != nil {
			// For other errors, set the HTTP status code to 503 (Service Unavailable)
			w.WriteHeader(http.StatusServiceUnavailable)
			// Log the error for further investigation
			fmt.Println("Hystrix circuit opened forAPI gateway with error:", err)
		}
	}
}

// makeProxyRequest is a helper function to make a proxy request using Hystrix.
func makeProxyRequest(lb *RoundRobinLoadBalancer, r *http.Request, targetURL string) (string, error) {
	var result string

	// Use Hystrix for circuit-breaking and timeout with the shared configuration
	err := hystrix.Do("shared", func() error {
		// Get the next server from the load balancer
		target := lb.NextServer()
		proxy := httputil.NewSingleHostReverseProxy(target)
		// Update the request URL to the target URL

		splitTargetURL := strings.Split(targetURL, "?")
		r.URL.Path = splitTargetURL[0]
		r.URL.Host = target.Host
		r.URL.Scheme = target.Scheme
		if len(splitTargetURL) > 1 {
			r.URL.RawQuery = splitTargetURL[1]
		}
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		r.Header.Set("Content-Type", "application/json")
		r.Host = target.Host

		// Capture the response to get the result
		recorder := httptest.NewRecorder()
		proxy.ServeHTTP(recorder, r)

		// Convert the recorded response to a string
		result = recorder.Body.String()

		return nil
	}, nil)

	if err != nil {
		return "", err
	}

	return result, nil
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
