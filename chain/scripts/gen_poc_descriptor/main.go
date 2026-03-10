// gen_poc_descriptor generates the gzipped FileDescriptorProto bytes for
// x/poc/types/tx.pb.go that includes all 10 Msg service methods plus the
// cosmos.msg.v1.service=true annotation required by MsgServiceRouter.
//
// Usage: go run ./scripts/gen_poc_descriptor/
package main

import (
	"bytes"
	"compress/gzip"
	"fmt"

	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func main() {
	// cosmos.msg.v1.service extension: field 11, wire type varint(0), value true(1)
	// tag = (11 << 3) | 0 = 88 = 0x58
	var serviceOptionsRaw []byte
	serviceOptionsRaw = appendVarint(serviceOptionsRaw, 88) // tag
	serviceOptionsRaw = appendVarint(serviceOptionsRaw, 1)  // value = true

	svcOpts := &descriptorpb.ServiceOptions{}
	svcOpts.ProtoReflect().SetUnknown(serviceOptionsRaw)

	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("pos/poc/v1/tx.proto"),
		Package: proto.String("pos.poc.v1"),
		Syntax:  proto.String("proto3"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("pos/x/poc/types"),
		},
		Dependency: []string{
			"amino/amino.proto",
			"cosmos/msg/v1/msg.proto",
			"cosmos_proto/cosmos.proto",
			"gogoproto/gogo.proto",
			"pos/poc/v1/params.proto",
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name:    proto.String("Msg"),
				Options: svcOpts,
				Method: []*descriptorpb.MethodDescriptorProto{
					{Name: proto.String("SubmitContribution"), InputType: proto.String(".pos.poc.v1.MsgSubmitContribution"), OutputType: proto.String(".pos.poc.v1.MsgSubmitContributionResponse")},
					{Name: proto.String("Endorse"), InputType: proto.String(".pos.poc.v1.MsgEndorse"), OutputType: proto.String(".pos.poc.v1.MsgEndorseResponse")},
					{Name: proto.String("WithdrawPOCRewards"), InputType: proto.String(".pos.poc.v1.MsgWithdrawPOCRewards"), OutputType: proto.String(".pos.poc.v1.MsgWithdrawPOCRewardsResponse")},
					{Name: proto.String("UpdateParams"), InputType: proto.String(".pos.poc.v1.MsgUpdateParams"), OutputType: proto.String(".pos.poc.v1.MsgUpdateParamsResponse")},
					{Name: proto.String("SubmitSimilarityCommitment"), InputType: proto.String(".pos.poc.v1.MsgSubmitSimilarityCommitment"), OutputType: proto.String(".pos.poc.v1.MsgSubmitSimilarityCommitmentResponse")},
					{Name: proto.String("StartReview"), InputType: proto.String(".pos.poc.v1.MsgStartReview"), OutputType: proto.String(".pos.poc.v1.MsgStartReviewResponse")},
					{Name: proto.String("CastReviewVote"), InputType: proto.String(".pos.poc.v1.MsgCastReviewVote"), OutputType: proto.String(".pos.poc.v1.MsgCastReviewVoteResponse")},
					{Name: proto.String("FinalizeReview"), InputType: proto.String(".pos.poc.v1.MsgFinalizeReview"), OutputType: proto.String(".pos.poc.v1.MsgFinalizeReviewResponse")},
					{Name: proto.String("AppealReview"), InputType: proto.String(".pos.poc.v1.MsgAppealReview"), OutputType: proto.String(".pos.poc.v1.MsgAppealReviewResponse")},
					{Name: proto.String("ResolveAppeal"), InputType: proto.String(".pos.poc.v1.MsgResolveAppeal"), OutputType: proto.String(".pos.poc.v1.MsgResolveAppealResponse")},
				},
			},
		},
	}

	b, err := proto.Marshal(fd)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(b)
	w.Close()

	gz := buf.Bytes()
	fmt.Printf("// %d bytes of a gzipped FileDescriptorProto\n", len(gz))
	for i, byt := range gz {
		if i%16 == 0 {
			fmt.Printf("\t")
		}
		fmt.Printf("0x%02x,", byt)
		if i%16 == 15 {
			fmt.Printf("\n")
		} else {
			fmt.Printf(" ")
		}
	}
	if len(gz)%16 != 0 {
		fmt.Printf("\n")
	}
}
