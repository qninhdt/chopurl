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

// SaveURL saves a URL to Cassandra
func (c *CassandraClient) SaveURL(urlEvent *URLEvent) error {
	// Insert the URL into the urls table
	query := "INSERT INTO urls (id, long_url, created_at) VALUES (?, ?, ?)"
	if err := c.session.Query(query, urlEvent.ID, urlEvent.LongURL, urlEvent.CreatedAt).Exec(); err != nil {
		return errors.New("failed to save URL to Cassandra: " + err.Error())
	}

	return nil
}
