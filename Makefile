.PHONY: build clean test proto run build-simapp compress-simapp

Version=$(shell git describe --tags --exact-match 2>/dev/null || echo "devel")
GitStatus=`git status -s`
GitCommit=`git rev-parse HEAD`
BuildTime=`date +%FT%T%z`
BuildGoVersion=`go version`

LDFLAGS=-ldflags "-w -s \
-X 'blazar/cmd.BinVersion=${Version}' \
-X 'blazar/cmd.GitStatus=${GitStatus}' \
-X 'blazar/cmd.GitCommit=${GitCommit}' \
-X 'blazar/cmd.BuildTime=${BuildTime}' \
-X 'blazar/cmd.BuildGoVersion=${BuildGoVersion}' \
"

build:
	go build -o blazar ${LDFLAGS}

run:
	go run ./...

clean:
	go clean

test:
	go test -mod=readonly -race ./...

lint:
	golangci-lint run ./...

format:
	go fmt ./...

proto:
	@ if ! which protoc > /dev/null; then \
		echo "error: protoc not installed" >&2; \
		exit 1; \
	fi
	protoc --proto_path=./proto --go_out=. --go-grpc_out=. --grpc-gateway_out=. --grpc-gateway_opt generate_unbound_methods=true proto/upgrades_registry.proto proto/version_resolver.proto proto/blazar.proto proto/checks.proto
	protoc-go-inject-tag -input="internal/pkg/proto/upgrades_registry/*.pb.go" -remove_tag_comment
	protoc-go-inject-tag -input="internal/pkg/proto/version_resolver/*.pb.go" -remove_tag_comment
	protoc-go-inject-tag -input="internal/pkg/proto/blazar/*.pb.go" -remove_tag_comment
	sed -i 's/upgrades_registry "internal\/pkg\/proto\/upgrades_registry"/upgrades_registry "blazar\/internal\/pkg\/proto\/upgrades_registry"/' internal/pkg/proto/version_resolver/version_resolver.pb.go

build-simapp:
	./testdata/scripts/build_simapp.sh

	cp testdata/scripts/start_simd_with_upgrade.sh ./testdata/daemon/images/v0.0.1/start_simd_with_upgrade.sh
	chmod +x ./testdata/daemon/images/v0.0.1/start_simd_with_upgrade.sh

compress-simapp:
	upx ./testdata/daemon/images/v0.0.1/simd-1
	upx ./testdata/daemon/images/v0.0.2/simd-2
