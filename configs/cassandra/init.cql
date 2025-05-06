CREATE KEYSPACE IF NOT EXISTS chopurl_keyspace
WITH REPLICATION = { 
    'class' : 'NetworkTopologyStrategy', 
    'DC1' : 3 
};