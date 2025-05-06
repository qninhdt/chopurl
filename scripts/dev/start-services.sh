#!/bin/bash

URL_SHORTEN_SERVICE_REPLICAS=1

docker compose up api-gateway url-shorten-service \
    --build \
    --scale url-shorten-service=${URL_SHORTEN_SERVICE_REPLICAS}
