#!/bin/sh
set -eux
docker build -f Dockerfile . -t strongbox8-builder
sudo modprobe fuse
docker run \
    --rm \
    --device /dev/fuse \
    -v "$PWD":/app \
    -w /app \
    --privileged \
    strongbox8-builder

