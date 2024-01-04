package tests

import (
	"testing"

	"github.com/klaytn/klaytn/api"
	"github.com/klaytn/klaytn/params"
)

func TestEth_Constants(t *testing.T) {
	// Simple constant values
	WithEth(t, nil, func(ctx *ethContext) {
		AssertEq(ctx, u64_0, "Hashrate")
		AssertEq(ctx, false, "Mining")
		AssertErr(ctx, "no mining work", "GetWork")
		AssertEq(ctx, false, "SubmitWork", api.BlockNonce{}, hash0, hash0)
		AssertEq(ctx, false, "SubmitHashrate", u64_0, hash0)
		AssertEq(ctx, api.ZeroHashrate, "GetHashrate")
		AssertEq(ctx, mapNil, "GetUncleByBlockNumberAndIndex", bg, num0, uint0)
		AssertEq(ctx, mapNil, "GetUncleByBlockHashAndIndex", bg, hash0, uint0)
		AssertEq(ctx, nodeAddr, "Etherbase")
		AssertEq(ctx, nodeAddr, "Coinbase")
	})
}

func TestEth_Uncle(t *testing.T) {
	// Uncle count
	refreshData(t, londonConfig)
	WithEth(t, []mockFunc{mockBlock}, func(ctx *ethContext) {
		AssertEq(ctx, &uint0, "GetUncleCountByBlockNumber", bg, numAny)
		AssertEq(ctx, &uint0, "GetUncleCountByBlockHash", bg, hashAny)
	})
	WithEth(t, []mockFunc{mockNoBlock}, func(ctx *ethContext) {
		AssertEq(ctx, uintNil, "GetUncleCountByBlockNumber", bg, numAny)
		AssertEq(ctx, uintNil, "GetUncleCountByBlockHash", bg, hashAny)
	})
}

func TestEth_Header(t *testing.T) {
	// No block
	WithEth(t, []mockFunc{mockNoBlock}, func(ctx *ethContext) {
		AssertEq(ctx, mapNil, "GetHeaderByNumber", bg, numAny)
		AssertEq(ctx, mapNil, "GetHeaderByHash", bg, hashAny)
		AssertEq(ctx, mapNil, "GetBlockByNumber", bg, numAny, true)
		AssertEq(ctx, mapNil, "GetBlockByHash", bg, hashAny, true)
	})

	// Blocks at different hardfork states
	configs := []*params.ChainConfig{
		londonConfig,
	}
	for _, config := range configs {
		refreshData(t, config)
		WithEth(t, []mockFunc{mockBlock}, func(ctx *ethContext) {
			AssertJson(ctx, ethHeaderMap, "GetHeaderByNumber", bg, numAny)
			AssertJson(ctx, ethHeaderMap, "GetHeaderByHash", bg, hashAny)

			AssertJson(ctx, ethBlockMapHash, "GetBlockByNumber", bg, numAny, false)
			AssertJson(ctx, ethBlockMapHash, "GetBlockByHash", bg, hashAny, false)

			AssertJson(ctx, ethBlockMapFull, "GetBlockByNumber", bg, numAny, true)
			AssertJson(ctx, ethBlockMapFull, "GetBlockByHash", bg, hashAny, true)
		})
	}
}
