#!/bin/bash
# Usage: ./scripts/switch-algorithm.sh [TokenBucket|LeakyBucket|FixedWindow|SlidingWindowCounter|SlidingWindowLog]

ALGO=${1:-"TokenBucket"}

echo "Settings algo to $ALGO"

grpcurl \
  -cacert ../certs/ca.crt \
  -cert ../certs/client.crt \
  -key ../certs/client.key \
  -proto ../api/v1/limiter.proto \
  -d "{\"algo\": \"$ALGO\"}" \
  127.0.0.1:50051 \
  sentinel.api.v1.RateLimiterService/UpdateAlgorithm

echo "Checking current algo..."

grpcurl \
  -cacert ../certs/ca.crt \
  -cert ../certs/client.crt \
  -key ../certs/client.key \
  -proto ../api/v1/limiter.proto \
  127.0.0.1:50051 \
  sentinel.api.v1.RateLimiterService/GetCurrentAlgorithm
