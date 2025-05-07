package main

import (
	"errors"
	"log"
	"time"

	"github.com/gocql/gocql"
)

type URLEvent struct {
	ID        int64     `json:"id"`
	LongURL   string    `json:"long_url"`
	CreatedAt time.Time `json:"created_at"`
}

// CassandraClient manages the connection and operations to Cassandra
type CassandraClient struct {
	session *gocql.Session
	options *CassandraOptions
}

// CassandraOptions holds configuration for Cassandra connection
type CassandraOptions struct {
	Hosts          []string      `mapstructure:"hosts"`
	Keyspace       string        `mapstructure:"keyspace"`
	Timeout        time.Duration `mapstructure:"timeout"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

// NewCassandraClient creates a new Cassandra client
func NewCassandraClient(options *CassandraOptions) (*CassandraClient, func(), error) {
	// Create a cluster config
	cluster := gocql.NewCluster(options.Hosts...)
	cluster.Keyspace = options.Keyspace
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = options.Timeout
	cluster.ConnectTimeout = options.ConnectTimeout

	// Create a session
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, nil, errors.New("failed to connect to Cassandra: " + err.Error())
	}

	log.Println("Connected to Cassandra cluster at", options.Hosts)

	// Create the Cassandra client
	cassandraClient := &CassandraClient{
		session: session,
		options: options,
	}

	return cassandraClient, func() {
		session.Close()
	}, nil
}

// GetURL retrieves a URL from Cassandra by its ID
func (c *CassandraClient) GetURL(id int64) (*URLEvent, error) {
	var urlEvent URLEvent
	query := "SELECT id, long_url, created_at FROM urls WHERE id = ? LIMIT 1"
	if err := c.session.Query(query, id).Scan(&urlEvent.ID, &urlEvent.LongURL, &urlEvent.CreatedAt); err != nil {
		if err == gocql.ErrNotFound {
			return nil, errors.New("URL not found")
		}
		return nil, errors.New("failed to get URL from Cassandra: " + err.Error())
	}

	return &urlEvent, nil
}
