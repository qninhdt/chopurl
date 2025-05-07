CREATE KEYSPACE IF NOT EXISTS chopurl_keyspace
WITH REPLICATION = { 
    'class' : 'NetworkTopologyStrategy', 
    'DC1' : 3 
};

USE chopurl_keyspace;

-- Simplified URLs table storing only ID, long URL and creation date
CREATE TABLE IF NOT EXISTS urls (
    id BIGINT PRIMARY KEY,
    long_url TEXT,
    created_at TIMESTAMP
);