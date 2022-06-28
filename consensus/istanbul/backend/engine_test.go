// Modifications Copyright 2020 The klaytn Authors
// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// This file is derived from quorum/consensus/istanbul/backend/engine_test.go (2020/04/16).
// Modified and improved for the klaytn development.

package backend

import (
	"bytes"
	"crypto/ecdsa"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/blockchain/vm"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/common/hexutil"
	"github.com/klaytn/klaytn/consensus"
	"github.com/klaytn/klaytn/consensus/istanbul"
	"github.com/klaytn/klaytn/consensus/istanbul/core"
	"github.com/klaytn/klaytn/crypto"
	"github.com/klaytn/klaytn/governance"
	"github.com/klaytn/klaytn/params"
	"github.com/klaytn/klaytn/reward"
	"github.com/klaytn/klaytn/rlp"
	"github.com/stretchr/testify/assert"
)

// These variables are the global variables of the test blockchain.
var (
	nodeKeys []*ecdsa.PrivateKey
	addrs    []common.Address
)

// These are the types in order to add a custom configuration of the test chain.
// You may need to create a configuration type if necessary.
type (
	istanbulCompatibleBlock  *big.Int
	LondonCompatibleBlock    *big.Int
	EthTxTypeCompatibleBlock *big.Int
	kip71CompatibleBlock     *big.Int
)

type (
	minimumStake           *big.Int
	stakingUpdateInterval  uint64
	proposerUpdateInterval uint64
	proposerPolicy         uint64
	governanceMode         string
	epoch                  uint64
	subGroupSize           uint64
	blockPeriod            uint64
)

// makeCommittedSeals returns a list of committed seals for the global variable nodeKeys.
func makeCommittedSeals(hash common.Hash) [][]byte {
	committedSeals := make([][]byte, len(nodeKeys))
	hashData := crypto.Keccak256(core.PrepareCommittedSeal(hash))
	for i, key := range nodeKeys {
		sig, _ := crypto.Sign(hashData, key)
		committedSeals[i] = make([]byte, types.IstanbulExtraSeal)
		copy(committedSeals[i][:], sig)
	}
	return committedSeals
}

// Include a node from the global nodeKeys and addrs
func includeNode(addr common.Address, key *ecdsa.PrivateKey) {
	for _, a := range addrs {
		if a.String() == addr.String() {
			// already exists
			return
		}
	}
	nodeKeys = append(nodeKeys, key)
	addrs = append(addrs, addr)
}

// Exclude a node from the global nodeKeys and addrs
func excludeNodeByAddr(target common.Address) {
	for i, a := range addrs {
		if a.String() == target.String() {
			nodeKeys = append(nodeKeys[:i], nodeKeys[i+1:]...)
			addrs = append(addrs[:i], addrs[i+1:]...)
			break
		}
	}
}

// in this test, we can set n to 1, and it means we can process Istanbul and commit a
// block by one node. Otherwise, if n is larger than 1, we have to generate
// other fake events to process Istanbul.
func newBlockChain(n int, items ...interface{}) (*blockchain.BlockChain, *backend) {
	// generate a genesis block
	genesis := blockchain.DefaultGenesisBlock()
	genesis.Config = params.TestChainConfig.Copy()
	genesis.Timestamp = uint64(time.Now().Unix())

	var (
		key    *ecdsa.PrivateKey
		period = istanbul.DefaultConfig.BlockPeriod
	)
	// force enable Istanbul engine and governance
	genesis.Config.Istanbul = params.GetDefaultIstanbulConfig()
	genesis.Config.Governance = params.GetDefaultGovernanceConfig()
	for _, item := range items {
		switch v := item.(type) {
		case istanbulCompatibleBlock:
			genesis.Config.IstanbulCompatibleBlock = v
		case LondonCompatibleBlock:
			genesis.Config.LondonCompatibleBlock = v
		case EthTxTypeCompatibleBlock:
			genesis.Config.EthTxTypeCompatibleBlock = v
		case kip71CompatibleBlock:
			genesis.Config.KIP71CompatibleBlock = v
		case proposerPolicy:
			genesis.Config.Istanbul.ProposerPolicy = uint64(v)
		case epoch:
			genesis.Config.Istanbul.Epoch = uint64(v)
		case subGroupSize:
			genesis.Config.Istanbul.SubGroupSize = uint64(v)
		case minimumStake:
			genesis.Config.Governance.Reward.MinimumStake = v
		case stakingUpdateInterval:
			genesis.Config.Governance.Reward.StakingUpdateInterval = uint64(v)
		case proposerUpdateInterval:
			genesis.Config.Governance.Reward.ProposerUpdateInterval = uint64(v)
		case governanceMode:
			genesis.Config.Governance.GovernanceMode = string(v)
		case *ecdsa.PrivateKey:
			key = v
		case blockPeriod:
			period = uint64(v)
		}
	}
	nodeKeys = make([]*ecdsa.PrivateKey, n)
	addrs = make([]common.Address, n)

	var b *backend
	if len(items) != 0 {
		b = newTestBackendWithConfig(genesis.Config, period, key)
	} else {
		b = newTestBackend()
	}

	nodeKeys[0] = b.privateKey
	addrs[0] = b.address // if governance mode is single, this address is the governing node address
	for i := 1; i < n; i++ {
		nodeKeys[i], _ = crypto.GenerateKey()
		addrs[i] = crypto.PubkeyToAddress(nodeKeys[i].PublicKey)
	}

	appendValidators(genesis, addrs)

	genesis.MustCommit(b.db)

	bc, err := blockchain.NewBlockChain(b.db, nil, genesis.Config, b, vm.Config{})
	if err != nil {
		panic(err)
	}
	if b.Start(bc, bc.CurrentBlock, bc.HasBadBlock) != nil {
		panic(err)
	}

	return bc, b
}

func appendValidators(genesis *blockchain.Genesis, addrs []common.Address) {
	if len(genesis.ExtraData) < types.IstanbulExtraVanity {
		genesis.ExtraData = append(genesis.ExtraData, bytes.Repeat([]byte{0x00}, types.IstanbulExtraVanity)...)
	}
	genesis.ExtraData = genesis.ExtraData[:types.IstanbulExtraVanity]

	ist := &types.IstanbulExtra{
		Validators:    addrs,
		Seal:          []byte{},
		CommittedSeal: [][]byte{},
	}

	istPayload, err := rlp.EncodeToBytes(&ist)
	if err != nil {
		panic("failed to encode istanbul extra")
	}
	genesis.ExtraData = append(genesis.ExtraData, istPayload...)
}

func makeHeader(parent *types.Block, config *istanbul.Config) *types.Header {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     parent.Number().Add(parent.Number(), common.Big1),
		GasUsed:    0,
		Extra:      parent.Extra(),
		Time:       new(big.Int).Add(parent.Time(), new(big.Int).SetUint64(config.BlockPeriod)),
		BlockScore: defaultBlockScore,
	}
	if parent.Header().BaseFee != nil {
		// We don't have chainConfig so the BaseFee of the current block is set by parent's for test
		header.BaseFee = parent.Header().BaseFee
	}
	return header
}

func makeBlock(chain *blockchain.BlockChain, engine *backend, parent *types.Block) *types.Block {
	block := makeBlockWithoutSeal(chain, engine, parent)
	stopCh := make(chan struct{})
	result, err := engine.Seal(chain, block, stopCh)
	if err != nil {
		panic(err)
	}
	return result
}

// makeBlockWithSeal creates a block with the proposer seal as well as all committed seals of validators.
func makeBlockWithSeal(chain *blockchain.BlockChain, engine *backend, parent *types.Block) *types.Block {
	blockWithoutSeal := makeBlockWithoutSeal(chain, engine, parent)

	// add proposer seal for the block
	block, err := engine.updateBlock(nil, blockWithoutSeal)
	if err != nil {
		panic(err)
	}

	// write validators committed seals to the block
	header := block.Header()
	committedSeals := makeCommittedSeals(block.Hash())
	err = writeCommittedSeals(header, committedSeals)
	if err != nil {
		panic(err)
	}
	block = block.WithSeal(header)

	return block
}

func makeBlockWithoutSeal(chain *blockchain.BlockChain, engine *backend, parent *types.Block) *types.Block {
	header := makeHeader(parent, engine.config)
	if err := engine.Prepare(chain, header); err != nil {
		panic(err)
	}
	state, _ := chain.StateAt(parent.Root())
	block, _ := engine.Finalize(chain, header, state, nil, nil)
	return block
}

func TestPrepare(t *testing.T) {
	chain, engine := newBlockChain(1)
	defer engine.Stop()

	header := makeHeader(chain.Genesis(), engine.config)
	err := engine.Prepare(chain, header)
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}

	header.ParentHash = common.HexToHash("0x1234567890")
	err = engine.Prepare(chain, header)
	if err != consensus.ErrUnknownAncestor {
		t.Errorf("error mismatch: have %v, want %v", err, consensus.ErrUnknownAncestor)
	}
}

func TestSealStopChannel(t *testing.T) {
	chain, engine := newBlockChain(4)
	defer engine.Stop()

	block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
	stop := make(chan struct{}, 1)
	eventSub := engine.EventMux().Subscribe(istanbul.RequestEvent{})
	eventLoop := func() {
		select {
		case ev := <-eventSub.Chan():
			_, ok := ev.Data.(istanbul.RequestEvent)
			if !ok {
				t.Errorf("unexpected event comes: %v", reflect.TypeOf(ev.Data))
			}
			stop <- struct{}{}
		}
		eventSub.Unsubscribe()
	}
	go eventLoop()

	finalBlock, err := engine.Seal(chain, block, stop)
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}

	if finalBlock != nil {
		t.Errorf("block mismatch: have %v, want nil", finalBlock)
	}
}

func TestSealCommitted(t *testing.T) {
	chain, engine := newBlockChain(1)
	defer engine.Stop()

	block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
	expectedBlock, _ := engine.updateBlock(engine.chain.GetHeader(block.ParentHash(), block.NumberU64()-1), block)

	actualBlock, err := engine.Seal(chain, block, make(chan struct{}))
	if err != nil {
		t.Errorf("error mismatch: have %v, want %v", err, expectedBlock)
	}

	if actualBlock.Hash() != expectedBlock.Hash() {
		t.Errorf("hash mismatch: have %v, want %v", actualBlock.Hash(), expectedBlock.Hash())
	}
}

func TestVerifyHeader(t *testing.T) {
	var configItems []interface{}
	configItems = append(configItems, istanbulCompatibleBlock(new(big.Int).SetUint64(0)))
	configItems = append(configItems, LondonCompatibleBlock(new(big.Int).SetUint64(0)))
	configItems = append(configItems, EthTxTypeCompatibleBlock(new(big.Int).SetUint64(0)))
	configItems = append(configItems, kip71CompatibleBlock(new(big.Int).SetUint64(0)))
	chain, engine := newBlockChain(1, configItems...)
	defer engine.Stop()

	// errEmptyCommittedSeals case
	block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
	block, _ = engine.updateBlock(chain.Genesis().Header(), block)
	err := engine.VerifyHeader(chain, block.Header(), false)
	if err != errEmptyCommittedSeals {
		t.Errorf("error mismatch: have %v, want %v", err, errEmptyCommittedSeals)
	}

	// short extra data
	header := block.Header()
	header.Extra = []byte{}
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidExtraDataFormat {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidExtraDataFormat)
	}
	// incorrect extra format
	header.Extra = []byte("0000000000000000000000000000000012300000000000000000000000000000000000000000000000000000000000000000")
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidExtraDataFormat {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidExtraDataFormat)
	}

	// invalid difficulty
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	header.BlockScore = big.NewInt(2)
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidBlockScore {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidBlockScore)
	}

	// invalid timestamp
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	header.Time = new(big.Int).Add(chain.Genesis().Time(), new(big.Int).SetUint64(engine.config.BlockPeriod-1))
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidTimestamp {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidTimestamp)
	}

	// future block
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	header.Time = new(big.Int).Add(big.NewInt(now().Unix()), new(big.Int).SetUint64(10))
	err = engine.VerifyHeader(chain, header, false)
	if err != consensus.ErrFutureBlock {
		t.Errorf("error mismatch: have %v, want %v", err, consensus.ErrFutureBlock)
	}

	// TODO-Klaytn: add more tests for header.Governance, header.Rewardbase, header.Vote
}

func TestVerifySeal(t *testing.T) {
	chain, engine := newBlockChain(1)
	defer engine.Stop()

	genesis := chain.Genesis()

	// cannot verify genesis
	err := engine.VerifySeal(chain, genesis.Header())
	if err != errUnknownBlock {
		t.Errorf("error mismatch: have %v, want %v", err, errUnknownBlock)
	}
	block := makeBlock(chain, engine, genesis)

	// clean cache before testing
	signatureAddresses.Purge()

	// change block content
	header := block.Header()
	header.Number = big.NewInt(4)
	block1 := block.WithSeal(header)
	err = engine.VerifySeal(chain, block1.Header())
	if err != errUnauthorized {
		t.Errorf("error mismatch: have %v, want %v", err, errUnauthorized)
	}

	// clean cache before testing
	signatureAddresses.Purge()

	// unauthorized users but still can get correct signer address
	engine.privateKey, _ = crypto.GenerateKey()
	err = engine.VerifySeal(chain, block.Header())
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}
}

func TestVerifyHeaders(t *testing.T) {
	chain, engine := newBlockChain(1)
	defer engine.Stop()

	genesis := chain.Genesis()

	// success case
	headers := []*types.Header{}
	blocks := []*types.Block{}
	size := 100

	for i := 0; i < size; i++ {
		var b *types.Block
		if i == 0 {
			b = makeBlockWithoutSeal(chain, engine, genesis)
			b, _ = engine.updateBlock(genesis.Header(), b)
			engine.db.WriteHeader(b.Header())
		} else {
			b = makeBlockWithoutSeal(chain, engine, blocks[i-1])
			b, _ = engine.updateBlock(blocks[i-1].Header(), b)
			engine.db.WriteHeader(b.Header())
		}
		blocks = append(blocks, b)
		headers = append(headers, blocks[i].Header())
	}

	// proceed time to avoid future block errors
	now = func() time.Time {
		return time.Unix(headers[size-1].Time.Int64(), 0)
	}
	defer func() {
		now = time.Now
	}()

	_, results := engine.VerifyHeaders(chain, headers, nil)
	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	index := 0
OUT1:
	for {
		select {
		case err := <-results:
			if err != nil {
				if err != errEmptyCommittedSeals && err != errInvalidCommittedSeals {
					t.Errorf("error mismatch: have %v, want errEmptyCommittedSeals|errInvalidCommittedSeals", err)
					break OUT1
				}
			}
			index++
			if index == size {
				break OUT1
			}
		case <-timeout.C:
			break OUT1
		}
	}
	// abort cases
	abort, results := engine.VerifyHeaders(chain, headers, nil)
	timeout = time.NewTimer(timeoutDura)
	index = 0
OUT2:
	for {
		select {
		case err := <-results:
			if err != nil {
				if err != errEmptyCommittedSeals && err != errInvalidCommittedSeals {
					t.Errorf("error mismatch: have %v, want errEmptyCommittedSeals|errInvalidCommittedSeals", err)
					break OUT2
				}
			}
			index++
			if index == 5 {
				abort <- struct{}{}
			}
			if index >= size {
				t.Errorf("verifyheaders should be aborted")
				break OUT2
			}
		case <-timeout.C:
			break OUT2
		}
	}
	// error header cases
	headers[2].Number = big.NewInt(100)
	abort, results = engine.VerifyHeaders(chain, headers, nil)
	timeout = time.NewTimer(timeoutDura)
	index = 0
	errors := 0
	expectedErrors := 2
OUT3:
	for {
		select {
		case err := <-results:
			if err != nil {
				if err != errEmptyCommittedSeals && err != errInvalidCommittedSeals {
					errors++
				}
			}
			index++
			if index == size {
				if errors != expectedErrors {
					t.Errorf("error mismatch: have %v, want %v", err, expectedErrors)
				}
				break OUT3
			}
		case <-timeout.C:
			break OUT3
		}
	}
}

func TestPrepareExtra(t *testing.T) {
	validators := make([]common.Address, 4)
	validators[0] = common.BytesToAddress(hexutil.MustDecode("0x44add0ec310f115a0e603b2d7db9f067778eaf8a"))
	validators[1] = common.BytesToAddress(hexutil.MustDecode("0x294fc7e8f22b3bcdcf955dd7ff3ba2ed833f8212"))
	validators[2] = common.BytesToAddress(hexutil.MustDecode("0x6beaaed781d2d2ab6350f5c4566a2c6eaac407a6"))
	validators[3] = common.BytesToAddress(hexutil.MustDecode("0x8be76812f765c24641ec63dc2852b378aba2b440"))

	vanity := make([]byte, types.IstanbulExtraVanity)
	expectedResult := append(vanity, hexutil.MustDecode("0xf858f8549444add0ec310f115a0e603b2d7db9f067778eaf8a94294fc7e8f22b3bcdcf955dd7ff3ba2ed833f8212946beaaed781d2d2ab6350f5c4566a2c6eaac407a6948be76812f765c24641ec63dc2852b378aba2b44080c0")...)

	h := &types.Header{
		Extra: vanity,
	}

	payload, err := prepareExtra(h, validators)
	if err != nil {
		t.Errorf("error mismatch: have %v, want: nil", err)
	}
	if !reflect.DeepEqual(payload, expectedResult) {
		t.Errorf("payload mismatch: have %v, want %v", payload, expectedResult)
	}

	// append useless information to extra-data
	h.Extra = append(vanity, make([]byte, 15)...)

	payload, err = prepareExtra(h, validators)
	if !reflect.DeepEqual(payload, expectedResult) {
		t.Errorf("payload mismatch: have %v, want %v", payload, expectedResult)
	}
}

func TestWriteSeal(t *testing.T) {
	vanity := bytes.Repeat([]byte{0x00}, types.IstanbulExtraVanity)
	istRawData := hexutil.MustDecode("0xf858f8549444add0ec310f115a0e603b2d7db9f067778eaf8a94294fc7e8f22b3bcdcf955dd7ff3ba2ed833f8212946beaaed781d2d2ab6350f5c4566a2c6eaac407a6948be76812f765c24641ec63dc2852b378aba2b44080c0")
	expectedSeal := append([]byte{1, 2, 3}, bytes.Repeat([]byte{0x00}, types.IstanbulExtraSeal-3)...)
	expectedIstExtra := &types.IstanbulExtra{
		Validators: []common.Address{
			common.BytesToAddress(hexutil.MustDecode("0x44add0ec310f115a0e603b2d7db9f067778eaf8a")),
			common.BytesToAddress(hexutil.MustDecode("0x294fc7e8f22b3bcdcf955dd7ff3ba2ed833f8212")),
			common.BytesToAddress(hexutil.MustDecode("0x6beaaed781d2d2ab6350f5c4566a2c6eaac407a6")),
			common.BytesToAddress(hexutil.MustDecode("0x8be76812f765c24641ec63dc2852b378aba2b440")),
		},
		Seal:          expectedSeal,
		CommittedSeal: [][]byte{},
	}
	var expectedErr error

	h := &types.Header{
		Extra: append(vanity, istRawData...),
	}

	// normal case
	err := writeSeal(h, expectedSeal)
	if err != expectedErr {
		t.Errorf("error mismatch: have %v, want %v", err, expectedErr)
	}

	// verify istanbul extra-data
	istExtra, err := types.ExtractIstanbulExtra(h)
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}
	if !reflect.DeepEqual(istExtra, expectedIstExtra) {
		t.Errorf("extra data mismatch: have %v, want %v", istExtra, expectedIstExtra)
	}

	// invalid seal
	unexpectedSeal := append(expectedSeal, make([]byte, 1)...)
	err = writeSeal(h, unexpectedSeal)
	if err != errInvalidSignature {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidSignature)
	}
}

func TestWriteCommittedSeals(t *testing.T) {
	vanity := bytes.Repeat([]byte{0x00}, types.IstanbulExtraVanity)
	istRawData := hexutil.MustDecode("0xf858f8549444add0ec310f115a0e603b2d7db9f067778eaf8a94294fc7e8f22b3bcdcf955dd7ff3ba2ed833f8212946beaaed781d2d2ab6350f5c4566a2c6eaac407a6948be76812f765c24641ec63dc2852b378aba2b44080c0")
	expectedCommittedSeal := append([]byte{1, 2, 3}, bytes.Repeat([]byte{0x00}, types.IstanbulExtraSeal-3)...)
	expectedIstExtra := &types.IstanbulExtra{
		Validators: []common.Address{
			common.BytesToAddress(hexutil.MustDecode("0x44add0ec310f115a0e603b2d7db9f067778eaf8a")),
			common.BytesToAddress(hexutil.MustDecode("0x294fc7e8f22b3bcdcf955dd7ff3ba2ed833f8212")),
			common.BytesToAddress(hexutil.MustDecode("0x6beaaed781d2d2ab6350f5c4566a2c6eaac407a6")),
			common.BytesToAddress(hexutil.MustDecode("0x8be76812f765c24641ec63dc2852b378aba2b440")),
		},
		Seal:          []byte{},
		CommittedSeal: [][]byte{expectedCommittedSeal},
	}
	var expectedErr error

	h := &types.Header{
		Extra: append(vanity, istRawData...),
	}

	// normal case
	err := writeCommittedSeals(h, [][]byte{expectedCommittedSeal})
	if err != expectedErr {
		t.Errorf("error mismatch: have %v, want %v", err, expectedErr)
	}

	// verify istanbul extra-data
	istExtra, err := types.ExtractIstanbulExtra(h)
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}
	if !reflect.DeepEqual(istExtra, expectedIstExtra) {
		t.Errorf("extra data mismatch: have %v, want %v", istExtra, expectedIstExtra)
	}

	// invalid seal
	unexpectedCommittedSeal := append(expectedCommittedSeal, make([]byte, 1)...)
	err = writeCommittedSeals(h, [][]byte{unexpectedCommittedSeal})
	if err != errInvalidCommittedSeals {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidCommittedSeals)
	}
}

func makeSnapshotTestConfigItems() []interface{} {
	return []interface{}{
		stakingUpdateInterval(1),
		proposerUpdateInterval(1),
		proposerPolicy(params.WeightedRandom),
	}
}

func makeFakeStakingInfo(blockNumber uint64, keys []*ecdsa.PrivateKey, amounts []uint64) *reward.StakingInfo {
	stakingInfo := &reward.StakingInfo{
		BlockNum: blockNumber,
	}
	for idx, key := range keys {
		addr := crypto.PubkeyToAddress(key.PublicKey)

		pk, _ := crypto.GenerateKey()
		rewardAddr := crypto.PubkeyToAddress(pk.PublicKey)

		stakingInfo.CouncilNodeAddrs = append(stakingInfo.CouncilNodeAddrs, addr)
		stakingInfo.CouncilStakingAmounts = append(stakingInfo.CouncilStakingAmounts, amounts[idx])
		stakingInfo.CouncilRewardAddrs = append(stakingInfo.CouncilRewardAddrs, rewardAddr)
	}
	return stakingInfo
}

func toAddressList(validators []istanbul.Validator) []common.Address {
	addresses := make([]common.Address, len(validators))
	for idx, val := range validators {
		addresses[idx] = val.Address()
	}
	return addresses
}

func copyAndSortAddrs(addrs []common.Address) []common.Address {
	copied := make([]common.Address, len(addrs))
	copy(copied, addrs)

	sort.Slice(copied, func(i, j int) bool {
		return strings.Compare(copied[i].String(), copied[j].String()) < 0
	})

	return copied
}

func makeExpectedResult(indices []int, candidate []common.Address) []common.Address {
	expected := make([]common.Address, len(indices))
	for eIdx, cIdx := range indices {
		expected[eIdx] = candidate[cIdx]
	}
	return copyAndSortAddrs(expected)
}

// Asserts taht if all (key,value) pairs of `subset` exists in `set`
func assertMapSubset(t *testing.T, subset, set map[string]interface{}, extraMsg ...interface{}) {
	for k, v := range subset {
		assert.Equal(t, set[k], v, extraMsg...)
	}
}

func TestChainConfig_UpdateAfterVotes(t *testing.T) {
	type vote struct {
		key   string
		value interface{}
	}
	type testcase struct {
		voting   vote
		expected vote
	}

	testcases := []testcase{
		{
			voting:   vote{"kip71.lowerboundbasefee", uint64(20000000000)}, // voted on block 1
			expected: vote{"kip71.lowerboundbasefee", uint64(20000000000)},
		},
		{
			voting:   vote{"kip71.upperboundbasefee", uint64(500000000000)}, // voted on block 1
			expected: vote{"kip71.upperboundbasefee", uint64(500000000000)},
		},
		{
			voting:   vote{"kip71.maxblockgasusedforbasefee", uint64(100000000)}, // voted on block 1
			expected: vote{"kip71.maxblockgasusedforbasefee", uint64(100000000)},
		},
		{
			voting:   vote{"kip71.gastarget", uint64(50000000)}, // voted on block 1
			expected: vote{"kip71.gastarget", uint64(50000000)},
		},
		{
			voting:   vote{"kip71.basefeedenominator", uint64(32)}, // voted on block 1
			expected: vote{"kip71.basefeedenominator", uint64(32)},
		},
	}

	var configItems []interface{}
	configItems = append(configItems, epoch(3))
	configItems = append(configItems, governanceMode("single"))
	configItems = append(configItems, blockPeriod(0)) // set block period to 0 to prevent creating future block
	for _, tc := range testcases {
		chain, engine := newBlockChain(1, configItems...)

		// test initial governance items
		assert.Equal(t, uint64(25000000000), chain.Config().Governance.KIP71.LowerBoundBaseFee)
		assert.Equal(t, uint64(750000000000), chain.Config().Governance.KIP71.UpperBoundBaseFee)
		assert.Equal(t, uint64(20), chain.Config().Governance.KIP71.BaseFeeDenominator)
		assert.Equal(t, uint64(60000000), chain.Config().Governance.KIP71.MaxBlockGasUsedForBaseFee)
		assert.Equal(t, uint64(30000000), chain.Config().Governance.KIP71.GasTarget)

		// add votes and insert voted blocks
		var (
			previousBlock, currentBlock *types.Block = nil, chain.Genesis()
			err                         error
		)

		engine.governance.SetBlockchain(chain)
		engine.governance.AddVote(tc.voting.key, tc.voting.value)
		previousBlock = currentBlock
		currentBlock = makeBlockWithSeal(chain, engine, previousBlock)
		_, err = chain.InsertChain(types.Blocks{currentBlock})
		assert.NoError(t, err)

		// insert blocks until the vote is applied
		for i := 0; i < 6; i++ {
			previousBlock = currentBlock
			currentBlock = makeBlockWithSeal(chain, engine, previousBlock)
			_, err = chain.InsertChain(types.Blocks{currentBlock})
			assert.NoError(t, err)
		}

		govConfig := chain.Config().Governance
		switch tc.expected.key {
		case "kip71.lowerboundbasefee":
			assert.Equal(t, tc.expected.value, govConfig.KIP71.LowerBoundBaseFee)
		case "kip71.upperboundbasefee":
			assert.Equal(t, tc.expected.value, govConfig.KIP71.UpperBoundBaseFee)
		case "kip71.gastarget":
			assert.Equal(t, tc.expected.value, govConfig.KIP71.GasTarget)
		case "kip71.maxblockgasusedforbasefee":
			assert.Equal(t, tc.expected.value, govConfig.KIP71.MaxBlockGasUsedForBaseFee)
		case "kip71.basefeedenominator":
			assert.Equal(t, tc.expected.value, govConfig.KIP71.BaseFeeDenominator)
		default:
			assert.Error(t, nil)
		}
	}
}

func TestChainConfig_ReadFromDBAfterVotes(t *testing.T) {
	type vote struct {
		key   string
		value interface{}
	}
	type testcase struct {
		voting   vote
		expected vote
	}

	testcases := []testcase{
		{
			voting:   vote{"kip71.lowerboundbasefee", uint64(20000000000)}, // voted on block 1
			expected: vote{"kip71.lowerboundbasefee", uint64(20000000000)},
		},
		{
			voting:   vote{"kip71.upperboundbasefee", uint64(500000000000)}, // voted on block 1
			expected: vote{"kip71.upperboundbasefee", uint64(500000000000)},
		},
		{
			voting:   vote{"kip71.maxblockgasusedforbasefee", uint64(100000000)}, // voted on block 1
			expected: vote{"kip71.maxblockgasusedforbasefee", uint64(100000000)},
		},
		{
			voting:   vote{"kip71.gastarget", uint64(50000000)}, // voted on block 1
			expected: vote{"kip71.gastarget", uint64(50000000)},
		},
		{
			voting:   vote{"kip71.basefeedenominator", uint64(32)}, // voted on block 1
			expected: vote{"kip71.basefeedenominator", uint64(32)},
		},
	}

	var configItems []interface{}
	configItems = append(configItems, proposerPolicy(params.WeightedRandom))
	configItems = append(configItems, epoch(3))
	configItems = append(configItems, governanceMode("single"))
	configItems = append(configItems, blockPeriod(0)) // set block period to 0 to prevent creating future block
	for _, tc := range testcases {
		chain, engine := newBlockChain(1, configItems...)

		// test initial governance items
		assert.Equal(t, uint64(25000000000), chain.Config().Governance.KIP71.LowerBoundBaseFee)
		assert.Equal(t, uint64(750000000000), chain.Config().Governance.KIP71.UpperBoundBaseFee)
		assert.Equal(t, uint64(20), chain.Config().Governance.KIP71.BaseFeeDenominator)
		assert.Equal(t, uint64(60000000), chain.Config().Governance.KIP71.MaxBlockGasUsedForBaseFee)
		assert.Equal(t, uint64(30000000), chain.Config().Governance.KIP71.GasTarget)

		// add votes and insert voted blocks
		var (
			previousBlock, currentBlock *types.Block = nil, chain.Genesis()
			err                         error
		)

		engine.governance.SetBlockchain(chain)
		engine.governance.AddVote(tc.voting.key, tc.voting.value)
		previousBlock = currentBlock
		currentBlock = makeBlockWithSeal(chain, engine, previousBlock)
		_, err = chain.InsertChain(types.Blocks{currentBlock})
		assert.NoError(t, err)

		// insert blocks until the vote is applied
		for i := 0; i < checkpointInterval; i++ {
			previousBlock = currentBlock
			currentBlock = makeBlockWithSeal(chain, engine, previousBlock)
			_, err = chain.InsertChain(types.Blocks{currentBlock})
			assert.NoError(t, err)
		}

		gov := governance.NewGovernanceInitialize(chain.Config(), engine.db)
		switch tc.expected.key {
		case "kip71.lowerboundbasefee":
			assert.Equal(t, tc.expected.value, gov.ChainConfig.Governance.KIP71.LowerBoundBaseFee)
		case "kip71.upperboundbasefee":
			assert.Equal(t, tc.expected.value, gov.ChainConfig.Governance.KIP71.UpperBoundBaseFee)
		case "kip71.gastarget":
			assert.Equal(t, tc.expected.value, gov.ChainConfig.Governance.KIP71.GasTarget)
		case "kip71.maxblockgasusedforbasefee":
			assert.Equal(t, tc.expected.value, gov.ChainConfig.Governance.KIP71.MaxBlockGasUsedForBaseFee)
		case "kip71.basefeedenominator":
			assert.Equal(t, tc.expected.value, gov.ChainConfig.Governance.KIP71.BaseFeeDenominator)
		default:
			assert.Error(t, nil)
		}

		engine.Stop()
	}
}
