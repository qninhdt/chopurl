#!/bin/bash

CASSANDRA_NODES=3
CASSANDRA_NON_SEED_NODES=$(($CASSANDRA_NODES - 1)) # exclude the seed node

# docker compose up cassandra-seed cassandra-node cassandra-init etcd redis-master redis-replica-1 redis-replica-2 \
#     --build \
#     --scale cassandra-node=${CASSANDRA_NON_SEED_NODES}

docker compose up etcd redis-master redis-replica-1 redis-replica-2 redis-sentinel \
    --build
