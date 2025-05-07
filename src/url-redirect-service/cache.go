package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheClient struct {
	redisClient *redis.Client
	options     *CacheOptions
}

type CacheOptions struct {
	SentinelAddress string        `mapstructure:"sentinel_address"` // sentinel address
	MasterName      string        `mapstructure:"master_name"`      // master name
	Password        string        `mapstructure:"password"`         // password
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`  // connect timeout in seconds
	SetTimeout      time.Duration `mapstructure:"set_timeout"`      // set timeout in seconds
}

func NewCacheClient(options *CacheOptions) (*CacheClient, func(), error) {
	// set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(options.ConnectTimeout)*time.Second)
	defer cancel()

	// create a new Redis client with sentinel support
	clientOptions := &redis.FailoverOptions{
		MasterName:    options.MasterName,
		SentinelAddrs: []string{options.SentinelAddress},
		Password:      options.Password,
	}

	client := redis.NewFailoverClient(clientOptions)

	// test the connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, nil, errors.New("failed to connect to Redis Sentinel: " + err.Error())
	}

	log.Println("Connected to Redis Sentinel at", options.SentinelAddress)

	cacheClient := &CacheClient{
		redisClient: client,
		options:     options,
	}

	return cacheClient, func() {
		if err := client.Close(); err != nil {
			log.Fatal("failed to close Redis client: " + err.Error())
		}
	}, nil
}

// GetURL retrieves a URL from the cache
func (c *CacheClient) GetURL(shortURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.options.SetTimeout)*time.Second)
	defer cancel()

	longURL, err := c.redisClient.Get(ctx, shortURL).Result()
	if err != nil {
		if err == redis.Nil {
			return "", errors.New("URL not found in cache")
		}
		return "", errors.New("failed to get URL from Redis: " + err.Error())
	}

	return longURL, nil
}
