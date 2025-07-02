#!/bin/sh

# Check if HTTP server is up
curl -sf http://localhost:9001/healthz || exit 1

# Check if /tmp is writable
echo test > /tmp/liveness-check || exit 1
rm /tmp/liveness-check

exit 0