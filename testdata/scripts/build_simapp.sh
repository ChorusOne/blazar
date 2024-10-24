#!/bin/bash

set -eu

BLAZAR_DIR=$PWD

echo "Cloning cosmos-sdk"
rm -rf /tmp/cosmos-sdk || true
git clone https://github.com/cosmos/cosmos-sdk.git /tmp/cosmos-sdk
cd /tmp/cosmos-sdk
git checkout v0.50.10

echo "Building cosmos-sdk"
make build

echo "Copy simapp as simd-1"
cp ./build/simd $BLAZAR_DIR/testdata/daemon/images/v0.0.1/simd-1

echo "Build and copy simapp as simd-2"
sed -i 's/const UpgradeName = "v047-to-v050"/const UpgradeName = "test1"/g' simapp/upgrades.go
sed -i simapp/upgrades.go -re "28,43d"
sed -i simapp/upgrades.go -re "6,7d"
make build
cp ./build/simd $BLAZAR_DIR/testdata/daemon/images/v0.0.2/simd-2

echo "Initializing simapp"
rm -r ~/.simapp || true
SIMD_BIN="./build/simd"

# source: cosmos-sdk/scripts/init-simapp.sh
if [ -z "$SIMD_BIN" ]; then echo "SIMD_BIN is not set. Make sure to run make install before"; exit 1; fi
echo "using $SIMD_BIN"
if [ -d "$($SIMD_BIN config home)" ]; then rm -rv $($SIMD_BIN config home); fi
$SIMD_BIN config set client chain-id demo
$SIMD_BIN config set client keyring-backend test
$SIMD_BIN config set app api.enable true
$SIMD_BIN keys add alice
$SIMD_BIN keys add bob
$SIMD_BIN init test --chain-id demo

echo "Modify voting period in genesis.json"
sed -i 's/"voting_period": "172800s",/"voting_period": "10s",/g' ~/.simapp/config/genesis.json
sed -i 's/"expedited_voting_period": "86400s",/"expedited_voting_period": "8s",/g' ~/.simapp/config/genesis.json

echo "Adding genesis accounts"
$SIMD_BIN genesis add-genesis-account alice 5000000000stake --keyring-backend test
$SIMD_BIN genesis add-genesis-account bob 5000000000stake --keyring-backend test
$SIMD_BIN genesis gentx alice 1000000stake --chain-id demo
$SIMD_BIN genesis collect-gentxs

echo "Configure cometbft settings"
sed -i 's/timeout_commit = "5s"/timeout_commit = "2s"/g' ~/.simapp/config/config.toml
sed -i 's/laddr = "tcp:\/\/127.0.0.1:26657"/laddr = "tcp:\/\/0.0.0.0:26657"/g' ~/.simapp/config/config.toml

sed -i 's/localhost:1317/0.0.0.0:1317/g' ~/.simapp/config/app.toml
sed -i 's/localhost:9090/0.0.0.0:9090/g' ~/.simapp/config/app.toml

echo "Copy .simapp to testdata/daemon/chain-home"
rm -rf $BLAZAR_DIR/testdata/daemon/chain-home || true
cp -r ~/.simapp $BLAZAR_DIR/testdata/daemon/chain-home
