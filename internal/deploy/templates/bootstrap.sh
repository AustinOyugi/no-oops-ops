#!/bin/sh
set -eu

old_ifs=$IFS
IFS=,
for mapping in ${NOOPS_SECRET_MAPPINGS:-}; do
  key=${mapping%%=*}
  path=${mapping#*=}
  [ -n "$key" ] || continue
  value=$(cat "$path")
  export "$key=$value"
done
IFS=$old_ifs

exec "$@"
