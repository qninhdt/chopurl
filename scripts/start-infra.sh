#!/bin/bash

docker compose up cassandra-1 cassandra-2 cassandra-3 cassandra-init etcd redis-sentinel redis-master redis-replica-1 redis-replica-2 \
    --build
