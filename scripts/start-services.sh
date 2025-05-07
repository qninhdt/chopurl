#!/bin/bash

URL_REDIRECT_SERVICE_REPLICAS=4
URL_SHORTEN_SERVICE_REPLICAS=4

docker compose up api-gateway url-shorten-service url-redirect-service \
    --build \
    --scale url-redirect-service=${URL_REDIRECT_SERVICE_REPLICAS} \
    --scale url-shorten-service=${URL_SHORTEN_SERVICE_REPLICAS}
