#!/bin/sh

# This server start listening connections by HTTP and pass it to StatsD by UDP

CURRENT_DIR=$(dirname $(readlink -f $0))

$CURRENT_DIR/../bin/statsd-http-proxy \
    --verbose \
    --http-host=127.0.0.1 \
    --http-port=433 \
    --tls-cert=cert.pem \
    --tls-key=key.pem \
    --statsd-host=127.0.0.1 \
    --statsd-port=8125 \
    --jwt-secret=somesecret \
    --metric-prefix=prefix.subprefix