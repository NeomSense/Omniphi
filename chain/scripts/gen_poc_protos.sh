#!/bin/bash
# Generate PoC module proto files

set -e

cd "$(dirname "$0")/.."

echo "🔧 Generating PoC proto files..."

# Get Cosmos SDK and gogoproto paths
COSMOS_SDK_DIR=$(go list -f '{{ .Dir }}' -m github.com/cosmos/cosmos-sdk)
GOGOPROTO_DIR=$(go list -f '{{ .Dir }}' -m github.com/cosmos/gogoproto)

# Generate each proto file individually
for proto_file in proto/pos/poc/v1/*.proto; do
    echo "  📄 Generating $(basename $proto_file)..."
    protoc \
        --proto_path=proto \
        --proto_path="$COSMOS_SDK_DIR/proto" \
        --proto_path="$GOGOPROTO_DIR" \
        --gocosmos_out=plugins=interfacetype+grpc,Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types:. \
        "$proto_file"
done

echo "✅ PoC proto generation complete!"
echo "📂 Generated files are in x/poc/types/"
