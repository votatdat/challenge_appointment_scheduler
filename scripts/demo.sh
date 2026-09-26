#!/bin/sh
# Requires a seeded, running server. Each invocation creates one appointment.
set -eu

BASE_URL=${BASE_URL:-http://localhost:${APP_PORT:-8080}}
START_TIME=${START_TIME:-$(date -u -d '+1 day' '+%Y-%m-%dT%H:%M:%SZ')}
# Allow only timestamp characters before constructing JSON. The API validates
# the actual RFC 3339 value and whether it is in the future.
case "$START_TIME" in
    ''|*[!0-9TZ:+.-]*) printf '%s\n' 'START_TIME must be an RFC 3339 timestamp.' >&2; exit 1 ;;
esac

demo_tmp=$(mktemp -d)
trap 'rm -rf "$demo_tmp"' EXIT HUP INT TERM
payload=$(printf '{"customer_id":1,"vehicle_id":1,"dealership_id":1,"service_type_id":1,"start_time":"%s"}' "$START_TIME")

status=$(curl --silent --show-error --max-time 15 \
    --dump-header "$demo_tmp/headers" --output "$demo_tmp/created" --write-out '%{http_code}' \
    --header 'Content-Type: application/json' --data "$payload" "${BASE_URL%/}/appointments")
printf 'POST /appointments -> %s\n' "$status"
cat "$demo_tmp/created"
if [ "$status" != 201 ]; then
    printf '%s\n' 'Expected 201; check seed data or choose another future START_TIME.' >&2
    exit 1
fi

location=$(awk 'tolower($1) == "location:" {gsub("\r", "", $2); print $2}' "$demo_tmp/headers")
id=${location#/appointments/}
case "$id" in
    ''|*[!0-9]*) printf '%s\n' 'Invalid Location response.' >&2; exit 1 ;;
esac
status=$(curl --silent --show-error --max-time 15 \
    --output "$demo_tmp/retrieved" --write-out '%{http_code}' "${BASE_URL%/}/appointments/$id")
printf 'GET /appointments/%s -> %s\n' "$id" "$status"
cat "$demo_tmp/retrieved"
[ "$status" = 200 ]
cmp -s "$demo_tmp/created" "$demo_tmp/retrieved"
printf '%s\n' 'Created and retrieved appointment representations match.'
