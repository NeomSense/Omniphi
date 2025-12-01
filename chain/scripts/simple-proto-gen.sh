#!/bin/bash
# Simplified proto generation - just gocosmos and go-grpc
set -e

echo "Generating query proto (gocosmos + grpc only)..."

# Get the protoc-gen tools from go bin
export PATH="$GOPATH/bin:$PATH"

# Use buf with just the essential plugins
cat > buf.gen.simple.yaml <<EOF
version: v2
managed:
  enabled: false
plugins:
  - local: protoc-gen-gocosmos
    out: .
    opt:
      - paths=source_relative
      - Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types
  - local: protoc-gen-go-grpc
    out: .
    opt:
      - paths=source_relative
EOF

# Generate with buf using simplified config
buf generate --template buf.gen.simple.yaml proto/pos/tokenomics/v1/query.proto

# Move files
if [ -d "pos/tokenomics/v1" ]; then
  cp pos/tokenomics/v1/query.pb.go x/tokenomics/types/ 2>/dev/null || true
  cp pos/tokenomics/v1/query_grpc.pb.go x/tokenomics/types/ 2>/dev/null || true
  rm -rf pos/
fi

# Clean up
rm -f buf.gen.simple.yaml

echo "✅ Done! Generated files copied to x/tokenomics/types/"
