package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var (
	redisClient *redis.Client
	ctx         = context.Background()
)

func main() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Initialize Redis with Sentinel
	initRedisWithSentinel()

	// Set up Gin router
	// Set up Gin router
	router := gin.Default()

	// Define routes
	router.GET("/health", healthCheckHandler)
	router.POST("/shorten", shortenURLHandler)
	router.GET("/:shortCode", redirectHandler)

	// Start server
	port := getEnv("PORT", "8080")
	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initRedisWithSentinel() {
	// Get Redis configuration from environment variables
	sentinelAddrs := strings.Split(getEnv("REDIS_SENTINEL_ADDRS", "redis-sentinel:26379"), ",")
	masterName := getEnv("REDIS_MASTER_NAME", "mymaster")
	password := getEnv("REDIS_PASSWORD", "")

	// Create a new Redis Sentinel client
	failoverClient := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    masterName,
		SentinelAddrs: sentinelAddrs,
		Password:      password,
		DB:            0,

		// Connection pool settings
		PoolSize:     10,
		MinIdleConns: 5,

		// Timeouts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test the connection
	_, err := failoverClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis Sentinel: %v", err)
	}

	log.Println("Successfully connected to Redis via Sentinel")
	redisClient = failoverClient
}

// Health check handler for the service
func healthCheckHandler(c *gin.Context) {
	// Verify Redis connection
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Redis health check failed: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Service is healthy",
	})
}

// Handler for shortening URLs
func shortenURLHandler(c *gin.Context) {
	var request struct {
		URL string `json:"url" binding:"required,url"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid URL format",
		})
		return
	}

	// Generate a short code (implementation not shown)
	shortCode := generateShortCode()

	// Store the mapping in Redis with an expiration time (1 year)
	err := redisClient.Set(ctx, shortCode, request.URL, 365*24*time.Hour).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to store URL",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"shortCode": shortCode,
		"shortUrl":  fmt.Sprintf("%s/%s", getBaseURL(c), shortCode),
	})
}

// Handler for redirecting to the original URL
func redirectHandler(c *gin.Context) {
	shortCode := c.Param("shortCode")

	// Get the original URL from Redis
	url, err := redisClient.Get(ctx, shortCode).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "URL not found",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve URL",
		})
		return
	}

	// Redirect to the original URL
	c.Redirect(http.StatusFound, url)
}

// Helper function to generate a short code
func generateShortCode() string {
	// random 7 character string use rand.Intn
	characters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	shortCode := make([]byte, 7)
	for i := range shortCode {
		shortCode[i] = characters[rand.Intn(len(characters))]
	}
	return string(shortCode)
}

// Get base URL from the request
func getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

// Get environment variable with fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
