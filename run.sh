#!/usr/bin/env bash

if [ $# -lt 2 ]; then
    echo "usage: run.sh <workers> <url>"
    exit 1
fi

key=$( head /dev/urandom | tr -dc A-Za-z0-9 | head -c32 )

# start the service prover in the background
./services/register_services $key &
serv_pid=$!

# run the crawler in the foreground
ww cluster run ./wasm/crawler.wasm $key ${@:1} || true

kill $serv_pid

