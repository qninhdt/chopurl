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
	}

	return cacheClient, func() {
		if err := client.Close(); err != nil {
			log.Fatal("failed to close Redis client: " + err.Error())
		}
	}, nil
}

// AddURL adds a URL to the cache with a specified expiration time.
func (c *CacheClient) AddURL(shortUrl string, longUrl string, expiration time.Duration) error {
	err := c.redisClient.Set(context.Background(), shortUrl, longUrl, expiration).Err()
	if err != nil {
		return errors.New("failed to set value in Redis: " + err.Error())
	}

	return nil
}
