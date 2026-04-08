#!/bin/bash

docker buildx build -f Dockerfile --platform linux/arm64 -t sentinel-go:latest --load .
