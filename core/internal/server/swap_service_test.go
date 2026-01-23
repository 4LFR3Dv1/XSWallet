// Package server - Tests for swap service mappings
package server

import (
	"testing"

	"github.com/xs-wallet/xscore/internal/provider"
	pb "github.com/xs-wallet/xscore/proto"
)

func TestProtoKindToProvider(t *testing.T) {
	cases := []struct {
		in  pb.SwapKind
		out provider.SwapKind
	}{
		{pb.SwapKind_SWAP_KIND_SUBMARINE, provider.SwapKindSubmarine},
		{pb.SwapKind_SWAP_KIND_REVERSE, provider.SwapKindReverse},
		{pb.SwapKind_SWAP_KIND_CHAIN, provider.SwapKindChain},
	}

	for _, c := range cases {
		if got := protoKindToProvider(c.in); got != c.out {
			t.Fatalf("kind %v -> %v, want %v", c.in, got, c.out)
		}
	}
}

func TestProtoChainToProvider(t *testing.T) {
	cases := []struct {
		in  pb.Chain
		out provider.Chain
	}{
		{pb.Chain_CHAIN_BTC, provider.ChainBTC},
		{pb.Chain_CHAIN_LN, provider.ChainLN},
		{pb.Chain_CHAIN_LIQUID, provider.ChainLiquid},
	}

	for _, c := range cases {
		if got := protoChainToProvider(c.in); got != c.out {
			t.Fatalf("chain %v -> %v, want %v", c.in, got, c.out)
		}
	}
}
