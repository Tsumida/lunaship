#!/bin/bash
set -euo pipefail

REDIS_CONF_DIR="/home/tsumida/code/github/lunaship/tmp/redis"
REDIS_NAME="redis-dev"
REDIS_IMG="redis:6.0"

docker run \
    -d \
    -p 16379:6379 \
    -v $REDIS_CONF_DIR:/usr/local/etc/redis \
    --name $REDIS_NAME \
    $REDIS_IMG \
    redis-server /usr/local/etc/redis/redis.conf
