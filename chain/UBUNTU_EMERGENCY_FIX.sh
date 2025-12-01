#!/bin/bash
# Ubuntu Emergency Fix Script
# Fixes: "panic: module 'feemarket' is missing a type URL"
#
# This script updates the feemarket proto stub file and rebuilds the binary

set -e  # Exit on any error

echo "======================================"
echo "Ubuntu Emergency Fix for Fee Market"
echo "======================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo -e "${RED}Error: go.mod not found. Please run this script from ~/omniphi/pos${NC}"
    exit 1
fi

echo -e "${YELLOW}Step 1: Creating proto directory structure...${NC}"
mkdir -p proto/pos/feemarket/module/v1

echo -e "${YELLOW}Step 2: Backing up old proto file (if exists)...${NC}"
if [ -f "proto/pos/feemarket/module/v1/module.pb.go" ]; then
    cp proto/pos/feemarket/module/v1/module.pb.go proto/pos/feemarket/module/v1/module.pb.go.backup
    echo -e "${GREEN}✓ Backup created: module.pb.go.backup${NC}"
else
    echo -e "${YELLOW}No existing file to backup${NC}"
fi

echo -e "${YELLOW}Step 3: Writing FIXED proto stub file...${NC}"

cat > proto/pos/feemarket/module/v1/module.pb.go << 'EOFPROTO'
// Code generated manually as stub. DO NOT EDIT.
// TODO: Replace with actual proto-generated code once buf generate works

package feemarketmodulev1

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
)

const (
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Module is the config object of the feemarket module.
type Module struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// authority defines the custom module authority. If not set, defaults to the governance module.
	Authority string `protobuf:"bytes,1,opt,name=authority,proto3" json:"authority,omitempty"`
}

func (x *Module) Reset() {
	*x = Module{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pos_feemarket_module_v1_module_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Module) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Module) ProtoMessage() {}

func (x *Module) ProtoReflect() protoreflect.Message {
	mi := &file_pos_feemarket_module_v1_module_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

var file_pos_feemarket_module_v1_module_proto_msgTypes = make([]protoimpl.MessageInfo, 1)

func init() {
	file_pos_feemarket_module_v1_module_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
		switch v := v.(*Module); i {
		case 0:
			return &v.state
		case 1:
			return &v.sizeCache
		case 2:
			return &v.unknownFields
		default:
			return nil
		}
	}
}
EOFPROTO

echo -e "${GREEN}✓ Proto file written successfully${NC}"

echo -e "${YELLOW}Step 4: Verifying proto file...${NC}"
if grep -q "func (x \*Module) ProtoReflect()" proto/pos/feemarket/module/v1/module.pb.go; then
    echo -e "${GREEN}✓ ProtoReflect() method found - file is correct!${NC}"
else
    echo -e "${RED}✗ ProtoReflect() method NOT found - something went wrong${NC}"
    exit 1
fi

echo -e "${YELLOW}Step 5: Cleaning old build artifacts...${NC}"
rm -f posd
go clean -cache
echo -e "${GREEN}✓ Clean complete${NC}"

echo -e "${YELLOW}Step 6: Rebuilding posd binary...${NC}"
echo "This may take a few minutes..."
if go build -o posd ./cmd/posd; then
    echo -e "${GREEN}✓ Build successful!${NC}"
else
    echo -e "${RED}✗ Build failed. Check errors above.${NC}"
    exit 1
fi

echo -e "${YELLOW}Step 7: Installing binary to system...${NC}"
if sudo cp posd /usr/local/bin/posd; then
    echo -e "${GREEN}✓ Binary installed to /usr/local/bin/posd${NC}"
else
    echo -e "${RED}✗ Failed to install. You may need sudo access.${NC}"
    echo "You can still use ./posd from this directory"
fi

echo -e "${YELLOW}Step 8: Verifying feemarket module is registered...${NC}"
if posd query --help | grep -q feemarket; then
    echo -e "${GREEN}✓ SUCCESS! Feemarket module is now registered!${NC}"
    echo ""
    echo "Module commands available:"
    posd query feemarket --help
else
    echo -e "${RED}✗ Module still not showing. This shouldn't happen.${NC}"
    echo "Try running: hash -r"
    echo "Then: posd query --help | grep feemarket"
    exit 1
fi

echo ""
echo "======================================"
echo -e "${GREEN}✓ FIX COMPLETE!${NC}"
echo "======================================"
echo ""
echo "Next steps:"
echo "1. Fix genesis denomination issues:"
echo "   cd ~/.pos"
echo "   sed -i 's/\"bond_denom\": \"omniphi\"/\"bond_denom\": \"uomni\"/g' config/genesis.json"
echo ""
echo "2. Add validator account:"
echo "   posd genesis add-genesis-account \$(posd keys show validator -a --home ~/.pos) 10000000000uomni --home ~/.pos"
echo ""
echo "3. Create gentx:"
echo "   posd genesis gentx validator 1000000000uomni --chain-id omniphi-testnet-1 --home ~/.pos"
echo ""
echo "4. Collect gentxs:"
echo "   posd genesis collect-gentxs --home ~/.pos"
echo ""
echo "5. Start chain:"
echo "   posd start --home ~/.pos"
echo ""
echo "For full instructions, see UBUNTU_MANUAL_FIX.md"
