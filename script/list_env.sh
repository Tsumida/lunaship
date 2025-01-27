#!/bin/bash

extract_env_vars() {
    local file="$1"
    grep -oP 'os\.Getenv\(\K[^)]+' "$file" | sed "s/['\"]//g"
}

export -f extract_env_vars
env_vars=$(find . -type f -name "*.go" -exec bash -c 'extract_env_vars "$0"' {} \; | sort | uniq)

for var in $env_vars; do
    value="${!var}" 
    echo "$var=$value"
done