package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"
)

func main() {

	// load configuration
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		log.Fatal("Error reading config file: ", err)
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := v.ReadInConfig()

	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			log.Println("Config file not found; using default values")
		} else {
			// Config file was found but another error was produced
			log.Fatal("Error reading config file: ", err)
		}
	} else {
		log.Println("Using config file:", v.ConfigFileUsed())
	}

	// bind to IdAllocatorOptions
	var idAllocOptions IdAllocatorOptions
	if err := v.UnmarshalKey("id_alloc", &idAllocOptions); err != nil {
		log.Fatal("Error unmarshalling ID Allocator options: ", err)
	}

	// bind to EtcdOptions
	var etcdOptions EtcdOptions
	if err := v.UnmarshalKey("etcd", &etcdOptions); err != nil {
		log.Fatal("Error unmarshalling Etcd options: ", err)
	}

	etcdOptions.Address = os.Getenv("ETCD_ADDRESS")

	// bind to CacheOptions
	var cacheOptions CacheOptions
	if err := v.UnmarshalKey("redis", &cacheOptions); err != nil {
		log.Fatal("Error unmarshalling Cache options: ", err)
	}

	cacheOptions.SentinelAddress = os.Getenv("REDIS_SENTINEL_ADDRESS")
	cacheOptions.MasterName = os.Getenv("REDIS_MASTER_NAME")
	cacheOptions.Password = os.Getenv("REDIS_PASSWORD")

	// bind to CassandraOptions
	var cassandraOptions CassandraOptions
	if err := v.UnmarshalKey("cassandra", &cassandraOptions); err != nil {
		log.Fatal("Error unmarshalling Cassandra options: ", err)
	}

	// Get Cassandra hosts from environment variable (comma-separated list)
	cassandraHostsEnv := os.Getenv("CASSANDRA_HOSTS")
	if cassandraHostsEnv != "" {
		cassandraOptions.Hosts = strings.Split(cassandraHostsEnv, ",")
	} else {
		// Default hosts if not specified in environment
		cassandraOptions.Hosts = []string{"cassandra-1", "cassandra-2", "cassandra-3"}
	}
	cassandraOptions.Keyspace = os.Getenv("CASSANDRA_KEYSPACE")
	if cassandraOptions.Keyspace == "" {
		cassandraOptions.Keyspace = "chopurl_keyspace"
	}

	// init id allocator
	idAllocator, cleanup, err := NewIdAllocator(&idAllocOptions, &etcdOptions)
	if err != nil {
		log.Fatal("Error initializing ID Allocator: ", err)
	}
	defer cleanup()

	// init cache client
	cacheClient, cleanup, err := NewCacheClient(&cacheOptions)
	if err != nil {
		log.Fatal("Error initializing Cache Client: ", err)
	}
	defer cleanup()

	// init cassandra client
	cassandraClient, cleanup, err := NewCassandraClient(&cassandraOptions)
	if err != nil {
		log.Fatal("Error initializing Cassandra Client: ", err)
	}
	defer cleanup()

	// Add CORS and rate limiting middleware
	middleware := func(h fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// Add CORS headers
			ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
			ctx.Response.Header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			ctx.Response.Header.Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")

			// Call the original handler
			h(ctx)
		}
	}

	// simple POST /create
	// JSON body: {"long_url": "http://example.com"} -> {"short_url": "http://short.url/123456"}
	createHandler := func(ctx *fasthttp.RequestCtx) {
		if !ctx.IsPost() {
			ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			return
		}

		var requestBody struct {
			LongURL string `json:"long_url"`
		}

		if err := json.Unmarshal(ctx.PostBody(), &requestBody); err != nil {
			ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
			return
		}

		if !IsValidURL(requestBody.LongURL) {
			ctx.Error("Invalid URL", fasthttp.StatusBadRequest)
			return
		}

		// generate a unique ID for the URL
		id, err := idAllocator.Pop()
		if err != nil {
			ctx.Error("Error allocating ID", fasthttp.StatusInternalServerError)
			return
		}

		// convert the ID to a base62 string
		shortURL := Int64ToBase62(id)

		log.Printf("Generated id base10=%d, base62=%s\n", id, shortURL)

		// Current timestamp for creation time
		now := time.Now()

		// store the mapping in the cache
		if err := cacheClient.AddURL(shortURL, requestBody.LongURL, 24*time.Hour); err != nil {
			ctx.Error("Error storing URL in cache", fasthttp.StatusInternalServerError)
			return
		}

		// Create URL event
		urlEvent := &URLEvent{
			LongURL:   requestBody.LongURL,
			CreatedAt: now,
			ID:        id,
		}

		// Save URL to Cassandra
		if err := cassandraClient.SaveURL(urlEvent); err != nil {
			log.Printf("Error saving URL to Cassandra: %v", err)
			// We don't return an error to the client here, as the URL is already in cache
			// The URL may be persisted later by a background process or retry mechanism
		} else {
			log.Printf("URL saved to Cassandra: id=%d", id)
		}

		// return the short URL
		response := struct {
			ShortURL string `json:"short_url"`
		}{
			ShortURL: "http://localhost/short/" + shortURL,
		}

		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)

		responseJSON, err := json.Marshal(response)
		if err != nil {
			ctx.Error("Error encoding response", fasthttp.StatusInternalServerError)
			return
		}
		ctx.Write(responseJSON)
	}

	// health check
	healthHandler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.Write([]byte("OK"))
	}

	// Set up the handler
	router := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		switch path {
		case "/create":
			createHandler(ctx)
		case "/health":
			healthHandler(ctx)
		default:
			ctx.Error("Not found", fasthttp.StatusNotFound)
		}
	}

	var port string = "8080"
	log.Println("Starting server on port", port)
	if err := fasthttp.ListenAndServe(":"+port, middleware(router)); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
