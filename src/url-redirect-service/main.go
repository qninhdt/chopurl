package main

import (
	"log"
	"os"
	"strings"

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
		cassandraOptions.Hosts = []string{"localhost"} // Changed to localhost for local development
	}
	cassandraOptions.Keyspace = os.Getenv("CASSANDRA_KEYSPACE")
	if cassandraOptions.Keyspace == "" {
		cassandraOptions.Keyspace = "chopurl_keyspace"
	}

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
			ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			ctx.Response.Header.Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")

			// Call the original handler
			h(ctx)
		}
	}

	// redirect handler
	redirectHandler := func(ctx *fasthttp.RequestCtx) {
		if !ctx.IsGet() {
			ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			return
		}

		// Get the short URL ID from the path
		path := string(ctx.Path())
		parts := strings.Split(path, "/")
		if len(parts) != 3 || parts[1] != "short" {
			ctx.Error("Invalid URL format. Expected /short/:id", fasthttp.StatusBadRequest)
			return
		}

		shortURL := parts[2]
		if shortURL == "" {
			ctx.Error("Invalid URL", fasthttp.StatusBadRequest)
			return
		}

		// Try to get the URL from cache first
		longURL, err := cacheClient.GetURL(shortURL)
		if err != nil {
			// If not in cache, try to get from Cassandra
			id, err := Base62ToInt64(shortURL)
			if err != nil {
				ctx.Error("Invalid URL", fasthttp.StatusBadRequest)
				return
			}

			urlEvent, err := cassandraClient.GetURL(id)
			if err != nil {
				ctx.Error("URL not found", fasthttp.StatusNotFound)
				return
			}

			longURL = urlEvent.LongURL
		}

		// Redirect to the long URL
		ctx.Redirect(longURL, fasthttp.StatusMovedPermanently)
	}

	// health check
	healthHandler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.Write([]byte("OK"))
	}

	// Set up the handler
	router := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		switch {
		case path == "/health":
			healthHandler(ctx)
		case strings.HasPrefix(path, "/short/"):
			redirectHandler(ctx)
		default:
			ctx.Error("Not found", fasthttp.StatusNotFound)
		}
	}

	var port string = "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	log.Println("Starting server on port", port)
	if err := fasthttp.ListenAndServe(":"+port, middleware(router)); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
