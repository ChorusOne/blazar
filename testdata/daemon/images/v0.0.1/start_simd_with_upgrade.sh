#!/bin/bash

set -eu

SIMD=/usr/bin/simd
if [ ! -f $SIMD ]; then
  echo "simd binary not found under $SIMD"
  exit 1
fi

echo "Starting simd"
$SIMD "$@" &
PID=$!
trap "kill $PID" EXIT

echo "Waiting for simd to start"
sleep 3

echo "Registering upgrade at height 10"
$SIMD tx upgrade software-upgrade test1 --title="Test Proposal" --summary="testing" --deposit="100000000stake" --upgrade-height 10 --upgrade-info '{ "binaries": { "linux/amd64":"https://example.com/simd.zip?checksum=sha256:aec070645fe53ee3b3763059376134f058cc337247c978add178b6ccdfb0019f" } }' --from alice --no-validate -y
sleep 3

echo "Vote for upgrade"
$SIMD tx gov vote 1 yes --from alice -y
$SIMD tx gov vote 1 yes --from bob -y

wait "$PID"
