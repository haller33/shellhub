#!/bin/sh

SCRIPTS_DIR=$(dirname $(readlink -f $0))

$SCRIPTS_DIR/env.sh SHELLHUB > /usr/share/nginx/html/env.js

exec nginx-debug -g "daemon off;"
