#!/bin/bash
# Proto generation script for tokenomics module
# This generates Go code from proto files using protoc

set -e

echo "Generating tokenomics proto files..."

# Get module paths
COSMOS_SDK_PATH=$(go list -f '{{.Dir}}' -m github.com/cosmos/cosmos-sdk)
GOGOPROTO_PATH=$(go list -f '{{.Dir}}' -m github.com/cosmos/gogoproto)
COSMOS_PROTO_PATH=$(go list -f '{{.Dir}}' -m github.com/cosmos/cosmos-proto)

# Protoc plugins
GOCOSMOS=$(which protoc-gen-gocosmos || echo "$GOPATH/bin/protoc-gen-gocosmos")
GO_GRPC=$(which protoc-gen-go-grpc || echo "$GOPATH/bin/protoc-gen-go-grpc")
GRPC_GATEWAY=$(which protoc-gen-grpc-gateway || echo "$GOPATH/bin/protoc-gen-grpc-gateway")

# Generate query.proto
protoc \
  --plugin="$GOCOSMOS" \
  --plugin="$GO_GRPC" \
  --plugin="$GRPC_GATEWAY" \
  -I proto \
  -I "$COSMOS_SDK_PATH/proto" \
  -I "$GOGOPROTO_PATH" \
  -I "$COSMOS_PROTO_PATH/proto" \
  --gocosmos_out=. \
  --gocosmos_opt=paths=source_relative \
  --gocosmos_opt=Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=. \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=logtostderr=true \
  --grpc-gateway_opt=allow_repeated_fields_in_body=true \
  proto/pos/tokenomics/v1/query.proto

# Move generated files to correct location
mkdir -p x/tokenomics/types
if [ -f "pos/tokenomics/v1/query.pb.go" ]; then
  mv pos/tokenomics/v1/query.pb.go x/tokenomics/types/
fi
if [ -f "pos/tokenomics/v1/query_grpc.pb.go" ]; then
  mv pos/tokenomics/v1/query_grpc.pb.go x/tokenomics/types/
fi
if [ -f "pos/tokenomics/v1/query.pb.gw.go" ]; then
  mv pos/tokenomics/v1/query.pb.gw.go x/tokenomics/types/
fi

# Clean up
rm -rf pos/

echo "✅ Proto generation complete!"
