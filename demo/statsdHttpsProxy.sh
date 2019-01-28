#!/bin/sh

# This server start listening connections by HTTPS and pass it to StatsD by UDP

# generate self-signed cert and key with default subject
#openssl req -x509 -nodes -days 358000 -newkey rsa:2048 -keyout key.pem -out cert.pem -subj "/C=PL/ST=test/L=test/O=test/OU=test/CN=test"

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