#!/bin/bash

# this script is used for setting up a environment for testing with limited CPU and memory resources.

./build.sh
docker run \
    --rm \
    -p 8080:8080 \
    -v ../controly:/controly \
    --cpus 2 \
    --memory=1g \
    --memory-swap=1g \
    alpine /controly
