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
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/log"
	"github.com/klaytn/klaytn/params"
	"github.com/klaytn/klaytn/reward"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// votingTestSuite tests various governance related methods.
//
// Parameter getters
// - Params()
// - ParamsAt()
// - GetGovernanceValue() -- tested by CurrentSetCopy()
// - ReadGovernance()
// - snapshot()
//   - snap.Epoch, snap.Number, etc.
// Validator getters
// - snapshot()
//   - snap.ValSet.List()
//   - snap.ValSet.DemotedList()

type testVotes = map[string]interface{}
type testExpectedParams = map[string]interface{} // (subset of) governance items
type testExpectedValset struct {                 // indices of validators
	validators []int
	demoted    []int
}
type votingTestCase struct {
	configOverride []interface{}              // chainconfig overrides
	stakeAmounts   []uint64                   // staking amounts of each validator
	endBlock       int                        // simulate until head=endBlock
	votes          map[int]testVotes          // votes to add at block #N
	expectedParams map[int]testExpectedParams // params effective at block #N
	expectedValset map[int]testExpectedValset // valset effective at block #N
}

// A votingTestSuite contains one votingTestCase and temporary test environments
type votingTestSuite struct {
	suite.Suite

	tc *votingTestCase

	chain             *blockchain.BlockChain
	engine            *backend
	oldStakingManager *reward.StakingManager
	allNodeKeys       []*ecdsa.PrivateKey // All validators created by newBlockChain
	allAddrs          []common.Address
}

func newVotingTestSuite(tc *votingTestCase) *votingTestSuite {
	return &votingTestSuite{tc: tc}
}

// Runs before every TesteVotes()
func (s *votingTestSuite) SetupTest() {
	log.EnableLogForTest(log.LvlCrit, log.LvlError)

	// Create test blockchain
	s.chain, s.engine = newBlockChain(4, s.tc.configOverride...)

	// Save the global singleton
	s.oldStakingManager = reward.GetStakingManager()

	// Replace the global singleton
	stakingInfo := makeFakeStakingInfo(0, nodeKeys, s.tc.stakeAmounts)
	reward.SetTestStakingManagerWithStakingInfoCache(stakingInfo)

	// Backup global `nodeKeys` and `addrs`
	s.allNodeKeys = make([]*ecdsa.PrivateKey, len(nodeKeys))
	s.allAddrs = make([]common.Address, len(addrs))
	copy(s.allNodeKeys, nodeKeys)
	copy(s.allAddrs, addrs)
}

// Runs after every TesteVotes()
func (s *votingTestSuite) TearDownTest() {
	if s.engine != nil {
		s.engine.Stop()
	}
	if s.oldStakingManager != nil {
		// Restore the global singleton
		reward.SetTestStakingManager(s.oldStakingManager)
	}
}

// Run checks after block processing has stopped
func (s *votingTestSuite) TestVotes() {
	t, chain, engine, tc := s.T(), s.chain, s.engine, s.tc

	var previousBlock, currentBlock *types.Block = nil, chain.Genesis()

	// Run checks along with block processing
	// `num` is the head block number in each iteration.
	for num := 0; num <= tc.endBlock; num++ {
		// Check at 0..endBlock
		s.checkCurrentParams(num)      // usage: backend.Prepare()
		s.checkHistoricParams(num)     // usage: snapshot.apply()
		s.checkHistoricParams(num + 1) // usage: reward.DistributeBlockReward()
		s.checkSnapshot(num)           // usage: backend.verifySigner()

		// Build blocks 1..endBlock
		// Note that we're building (num+1)'th block here.
		if num == tc.endBlock {
			break
		}

		// Place a vote if a vote is scheduled in upcoming block
		for k, v := range tc.votes[num+1] {
			v = s.adjustVote(k, v)
			ok := engine.governance.AddVote(k, v)
			assert.True(t, ok)
			// t.Logf("Casted vote num=%d key=%s value=%v", num+1, k, v)
		}

		// Create a block with votes
		previousBlock = currentBlock
		currentBlock = makeBlockWithSeal(chain, engine, previousBlock)
		_, err := chain.InsertChain(types.Blocks{currentBlock})
		assert.Nil(t, err)
		// t.Logf("Created block num=%d", currentBlock.Number().Uint64())

		// Reflect votes to the globals if necessary
		for k, v := range tc.votes[num+1] {
			s.reflectValidatorVote(k, v)
		}
	}

	assert.Equal(t, uint64(tc.endBlock), chain.CurrentHeader().Number.Uint64())
	// t.Logf("Done building until num=%d", tc.endBlock)

	// Run checks after block processing has stopped
	// Test historical blocks as well as pending (endBlock+1) block.
	for num := 0; num <= tc.endBlock+1; num++ {
		s.checkHistoricParams(num) // usage: governance_itemsAt
		s.checkSnapshot(num)       // usage: istanbul_getSnapshot
	}
}

// Checks
// - Params()
// - GetGovernanceValue() -- tested by CurrentSetCopy()
func (s *votingTestSuite) checkCurrentParams(num int) {
	t, engine, tc := s.T(), s.engine, s.tc
	if expected, ok := tc.expectedParams[num]; ok {
		assertMapSubset(t, expected, engine.governance.Params().StrMap(), "Params() num=%d", num)
		assertMapSubset(t, expected, engine.governance.CurrentSetCopy(), "currentSet num=%d", num)
	}
}

// Checks
// - ParamsAt()
// - ReadGovernance()
func (s *votingTestSuite) checkHistoricParams(num int) {
	t, engine, tc := s.T(), s.engine, s.tc
	if expected, ok := tc.expectedParams[num]; ok {
		pset, err := engine.governance.ParamsAt(uint64(num))
		assert.NoError(t, err)
		assertMapSubset(t, expected, pset.StrMap(), "ParamsAt(num=%d)", num)

		_, items, err := engine.governance.ReadGovernance(uint64(num))
		assert.NoError(t, err)
		assertMapSubset(t, expected, items, "ReadGovernance(num=%d)", num)
	}
}

// Checks
// - snapshot()
//   - snap.Number, snap.Hash
//   - snap.Epoch, snap.Policy, snap.CommitteeSize
//   - snap.ValSet.List()
//   - snap.ValSet.DemotedList()
func (s *votingTestSuite) checkSnapshot(num int) {
	t, chain, engine, tc := s.T(), s.chain, s.engine, s.tc

	// Block #N is validated based on snapshot(N-1).
	var snapNum int
	if num == 0 {
		snapNum = 0
	} else {
		snapNum = num - 1
	}

	// Load snapshot at snapNum
	block := chain.GetBlockByNumber(uint64(snapNum))
	require.NotNil(t, block)
	snap, err := engine.snapshot(chain, block.NumberU64(), block.Hash(), nil)
	require.Nil(t, err)
	require.NotNil(t, snap)

	// Check the retrieved snapshot
	assert.Equal(t, block.NumberU64(), snap.Number)
	assert.Equal(t, block.Hash(), snap.Hash)

	// Note that snap.Epoch = snapshot(snapNum).Epoch = ParamsAt(snapNum).Epoch
	if expected, ok := tc.expectedParams[snapNum]; ok {
		if epoch, ok := expected["istanbul.epoch"]; ok {
			assert.Equal(t, epoch, snap.Epoch, "snapshot(num=%d)", snapNum)
		}
		if policy, ok := expected["istanbul.policy"]; ok {
			assert.Equal(t, policy, snap.Policy, "snapshot(num=%d)", snapNum)
		}
		if committeeSize, ok := expected["istanbul.committeesize"]; ok {
			assert.Equal(t, committeeSize, snap.CommitteeSize, "snapshot(num=%d)", snapNum)
		}
	}

	if expected, ok := tc.expectedValset[num]; ok {
		var (
			actualValidators   = copyAndSortAddrs(toAddressList(snap.ValSet.List()))
			actualDemoted      = copyAndSortAddrs(toAddressList(snap.ValSet.DemotedList()))
			expectedValidators = makeExpectedResult(expected.validators, s.allAddrs)
			expectedDemoted    = makeExpectedResult(expected.demoted, s.allAddrs)
		)
		assert.Equal(t, expectedValidators, actualValidators,
			"validators num=%d len(expected)=%d len(actual)=%d", num, len(expectedValidators), len(actualValidators))
		assert.Equal(t, expectedDemoted, actualDemoted,
			"demoted num=%d len(expected)=%d len(actual)=%d", num, len(expectedDemoted), len(actualDemoted))
	}
	// t.Logf("snapshot(%d) ValSet len=%d,%d", snapNum, len(snap.ValSet.List()), len(snap.ValSet.DemotedList()))
}

// In engine_test.go, some testVotes are expressed in node indices for convenience.
// Here we convert the indices back to node addresses.
func (s *votingTestSuite) adjustVote(key string, value interface{}) interface{} {
	t, allAddrs := s.T(), s.allAddrs

	switch key {
	case "governance.addvalidator", "governance.removevalidator": // int or []int
		if idx, ok := value.(int); ok {
			value = allAddrs[idx]
		} else if indices, ok := value.([]int); ok {
			value = makeExpectedResult(indices, allAddrs)
		} else {
			t.Fatal("Invalid vote value")
		}
	case "governance.governingnode": // int
		if idx, ok := value.(int); ok {
			value = allAddrs[idx]
		} else {
			t.Fatal("Invalid vote value")
		}
	}
	return value
}

// engine_test.go relies on two globals `[]nodeKeys` and `[]addrs`.
// Helper functions will iterate over the arrays by default.
// We must modify the globals to avoid errors like errInvalidCommittedSeals.
func (s *votingTestSuite) reflectValidatorVote(key string, value interface{}) {
	allNodeKeys, allAddrs := s.allNodeKeys, s.allAddrs

	var indicesDiff []int

	switch key {
	case "governance.addvalidator", "governance.removevalidator": // int or []int
		if idx, ok := value.(int); ok {
			indicesDiff = []int{idx}
		} else if indices, ok := value.([]int); ok {
			indicesDiff = indices
		}
	}

	switch key {
	case "governance.addvalidator":
		for _, i := range indicesDiff {
			includeNode(allAddrs[i], allNodeKeys[i])
		}
	case "governance.removevalidator":
		for _, i := range indicesDiff {
			excludeNodeByAddr(allAddrs[i])
		}
	}
}

// supercedes TestGovernance_ReaderEngine and TestGovernance_Votes
func TestGovernance_Params(t *testing.T) {
	var configItems []interface{}
	configItems = append(configItems, proposerPolicy(params.WeightedRandom))
	configItems = append(configItems, epoch(3))
	configItems = append(configItems, governanceMode("single"))
	configItems = append(configItems, blockPeriod(0)) // set block period to 0 to prevent creating future block
	stakes := []uint64{0, 0, 0, 0}

	testcases := []votingTestCase{
		{ // Simple parameter update timing
			configItems,
			stakes,
			7,
			map[int]testVotes{
				1: {"governance.unitprice": uint64(17)},
			},
			map[int]testExpectedParams{
				0: {"governance.unitprice": uint64(1)},
				1: {"governance.unitprice": uint64(1)},
				2: {"governance.unitprice": uint64(1)},
				3: {"governance.unitprice": uint64(1)},
				4: {"governance.unitprice": uint64(1)},
				5: {"governance.unitprice": uint64(1)},  // Epoch - 1
				6: {"governance.unitprice": uint64(17)}, // Epoch
				7: {"governance.unitprice": uint64(17)}, // Epoch + 1
			},
			nil,
		},
		{ // Repeating changes
			configItems,
			stakes,
			13,
			map[int]testVotes{
				1: {"governance.governancemode": "none"},   // effects from 6
				2: {"istanbul.committeesize": uint64(4)},   // effects from 6
				3: {"governance.unitprice": uint64(750)},   // effects from 9
				4: {"governance.governancemode": "single"}, // effects from 9
				5: {"istanbul.committeesize": uint64(22)},  // effects from 9
				6: {"governance.unitprice": uint64(2)},     // effects from 12
			},
			map[int]testExpectedParams{
				0:  {"governance.governancemode": "single", "istanbul.committeesize": uint64(21), "governance.unitprice": uint64(1)},
				1:  {"governance.governancemode": "single", "istanbul.committeesize": uint64(21), "governance.unitprice": uint64(1)},
				2:  {"governance.governancemode": "single", "istanbul.committeesize": uint64(21), "governance.unitprice": uint64(1)},
				3:  {"governance.governancemode": "single", "istanbul.committeesize": uint64(21), "governance.unitprice": uint64(1)},
				4:  {"governance.governancemode": "single", "istanbul.committeesize": uint64(21), "governance.unitprice": uint64(1)},
				5:  {"governance.governancemode": "single", "istanbul.committeesize": uint64(21), "governance.unitprice": uint64(1)},
				6:  {"governance.governancemode": "none", "istanbul.committeesize": uint64(4), "governance.unitprice": uint64(1)},
				7:  {"governance.governancemode": "none", "istanbul.committeesize": uint64(4), "governance.unitprice": uint64(1)},
				8:  {"governance.governancemode": "none", "istanbul.committeesize": uint64(4), "governance.unitprice": uint64(1)},
				9:  {"governance.governancemode": "single", "istanbul.committeesize": uint64(22), "governance.unitprice": uint64(750)},
				10: {"governance.governancemode": "single", "istanbul.committeesize": uint64(22), "governance.unitprice": uint64(750)},
				11: {"governance.governancemode": "single", "istanbul.committeesize": uint64(22), "governance.unitprice": uint64(750)},
				12: {"governance.governancemode": "single", "istanbul.committeesize": uint64(22), "governance.unitprice": uint64(2)},
				13: {"governance.governancemode": "single", "istanbul.committeesize": uint64(22), "governance.unitprice": uint64(2)},
			},
			nil,
		},
		{ // Voting items governance.*
			configItems,
			stakes,
			6,
			map[int]testVotes{
				1: {"governance.governancemode": "none"},
				2: {"governance.unitprice": uint64(2000000)},
			},
			map[int]testExpectedParams{
				0: {"governance.governancemode": "single", "governance.unitprice": uint64(1)},
				5: {"governance.governancemode": "single", "governance.unitprice": uint64(1)},
				6: {"governance.governancemode": "none", "governance.unitprice": uint64(2000000)},
			},
			nil,
		},
		{ // Voting items istanbul.*
			configItems,
			stakes,
			6,
			map[int]testVotes{
				1: {"istanbul.committeesize": uint64(4)},
			},
			map[int]testExpectedParams{
				0: {"istanbul.committeesize": uint64(21)},
				5: {"istanbul.committeesize": uint64(21)},
				6: {"istanbul.committeesize": uint64(4)},
			},
			nil,
		},
		{ // Voting items reward.*
			configItems,
			stakes,
			9,
			map[int]testVotes{
				1: {"reward.mintingamount": "12340000000"}, // effects from #6
				2: {"reward.ratio": "34/33/33"},            // effects from #6
				3: {"reward.useginicoeff": true},           // effects from #9
				4: {"reward.minimumstake": "1234000"},      // effects from #9
			},
			map[int]testExpectedParams{
				0: {"reward.mintingamount": "0", "reward.ratio": "100/0/0", "reward.useginicoeff": false, "reward.minimumstake": "2000000"},
				5: {"reward.mintingamount": "0", "reward.ratio": "100/0/0", "reward.useginicoeff": false, "reward.minimumstake": "2000000"},
				6: {"reward.mintingamount": "12340000000", "reward.ratio": "34/33/33", "reward.useginicoeff": false, "reward.minimumstake": "2000000"},
				8: {"reward.mintingamount": "12340000000", "reward.ratio": "34/33/33", "reward.useginicoeff": false, "reward.minimumstake": "2000000"},
				9: {"reward.mintingamount": "12340000000", "reward.ratio": "34/33/33", "reward.useginicoeff": true, "reward.minimumstake": "1234000"},
			},
			nil,
		},
	}

	for _, tc := range testcases {
		suite.Run(t, newVotingTestSuite(&tc))
	}
}

// supercedes TestSnapshot_Validators_AddRemove
// Tests voting items governance.addvalidator and governance.removevalidator
func TestGovernance_AddRemoveVals(t *testing.T) {
	var configItems []interface{}
	configItems = append(configItems, proposerPolicy(params.WeightedRandom))
	configItems = append(configItems, epoch(3))
	configItems = append(configItems, minimumStake(new(big.Int).SetUint64(4000000)))
	configItems = append(configItems, istanbulCompatibleBlock(new(big.Int).SetUint64(0)))
	configItems = append(configItems, blockPeriod(0)) // set block period to 0 to prevent creating future block
	stakes := []uint64{4000000, 4000000, 4000000, 4000000}

	testcases := []votingTestCase{
		{ // Singular change
			configItems,
			stakes,
			5,
			map[int]testVotes{
				1: {"governance.removevalidator": 3},
				3: {"governance.addvalidator": 3},
			},
			nil,
			map[int]testExpectedValset{
				0: {[]int{0, 1, 2, 3}, nil},
				1: {[]int{0, 1, 2, 3}, nil},
				2: {[]int{0, 1, 2}, nil},
				3: {[]int{0, 1, 2}, nil},
				4: {[]int{0, 1, 2, 3}, nil},
				5: {[]int{0, 1, 2, 3}, nil},
			},
		},
		{ // Plural change
			configItems,
			stakes,
			5,
			map[int]testVotes{
				1: {"governance.removevalidator": []int{1, 2, 3}},
				3: {"governance.addvalidator": []int{1, 2}},
			},
			nil,
			map[int]testExpectedValset{
				0: {[]int{0, 1, 2, 3}, nil},
				1: {[]int{0, 1, 2, 3}, nil},
				2: {[]int{0}, nil},
				3: {[]int{0}, nil},
				4: {[]int{0, 1, 2}, nil},
				5: {[]int{0, 1, 2}, nil},
			},
		},
		{ // Duplicating addvalidator then removevalidator (singular)
			configItems,
			stakes,
			11,
			map[int]testVotes{
				2:  {"governance.removevalidator": 3},
				4:  {"governance.addvalidator": 3},
				6:  {"governance.addvalidator": 3},
				8:  {"governance.removevalidator": 3},
				10: {"governance.removevalidator": 3},
			},
			nil,
			map[int]testExpectedValset{
				3:  {[]int{0, 1, 2}, nil},
				5:  {[]int{0, 1, 2, 3}, nil},
				7:  {[]int{0, 1, 2, 3}, nil},
				9:  {[]int{0, 1, 2}, nil},
				11: {[]int{0, 1, 2}, nil},
			},
		},
		{ // Duplicating removevalidator then addvalidator (singular)
			configItems,
			stakes,
			9,
			map[int]testVotes{
				2: {"governance.removevalidator": 3},
				4: {"governance.removevalidator": 3},
				6: {"governance.addvalidator": 3},
				8: {"governance.addvalidator": 3},
			},
			nil,
			map[int]testExpectedValset{
				3: {[]int{0, 1, 2}, nil},
				5: {[]int{0, 1, 2}, nil},
				7: {[]int{0, 1, 2, 3}, nil},
				9: {[]int{0, 1, 2, 3}, nil},
			},
		},
		{ // Duplicating addvalidator then removevalidator (plural)
			configItems,
			stakes,
			11,
			map[int]testVotes{
				2:  {"governance.removevalidator": []int{2, 3}},
				4:  {"governance.addvalidator": []int{2, 3}},
				6:  {"governance.addvalidator": []int{2, 3}},
				8:  {"governance.removevalidator": []int{2, 3}},
				10: {"governance.removevalidator": []int{2, 3}},
			},
			nil,
			map[int]testExpectedValset{
				3:  {[]int{0, 1}, nil},
				5:  {[]int{0, 1, 2, 3}, nil},
				7:  {[]int{0, 1, 2, 3}, nil},
				9:  {[]int{0, 1}, nil},
				11: {[]int{0, 1}, nil},
			},
		},
		{ // Duplicating removevalidator then addvalidator (plural)
			configItems,
			stakes,
			9,
			map[int]testVotes{
				2: {"governance.removevalidator": []int{2, 3}},
				4: {"governance.removevalidator": []int{2, 3}},
				6: {"governance.addvalidator": []int{2, 3}},
				8: {"governance.addvalidator": []int{2, 3}},
			},
			nil,
			map[int]testExpectedValset{
				3: {[]int{0, 1}, nil},
				5: {[]int{0, 1}, nil},
				7: {[]int{0, 1, 2, 3}, nil},
				9: {[]int{0, 1, 2, 3}, nil},
			},
		},
		{ // Around checkpoint interval (i.e. every 1024 block)
			configItems,
			stakes,
			checkpointInterval + 10,
			map[int]testVotes{
				checkpointInterval - 5: {"governance.removevalidator": 3},
				checkpointInterval - 1: {"governance.removevalidator": 2},
				checkpointInterval + 0: {"governance.removevalidator": 1},
				checkpointInterval + 1: {"governance.addvalidator": 1},
				checkpointInterval + 2: {"governance.addvalidator": 2},
				checkpointInterval + 3: {"governance.addvalidator": 3},
			},
			nil,
			map[int]testExpectedValset{
				0:                      {[]int{0, 1, 2, 3}, nil},
				1:                      {[]int{0, 1, 2, 3}, nil},
				checkpointInterval - 4: {[]int{0, 1, 2}, nil},
				checkpointInterval + 0: {[]int{0, 1}, nil},
				checkpointInterval + 1: {[]int{0}, nil},
				checkpointInterval + 2: {[]int{0, 1}, nil},
				checkpointInterval + 3: {[]int{0, 1, 2}, nil},
				checkpointInterval + 4: {[]int{0, 1, 2, 3}, nil},
				checkpointInterval + 9: {[]int{0, 1, 2, 3}, nil},
			},
		},
	}

	for _, tc := range testcases {
		suite.Run(t, newVotingTestSuite(&tc))
	}
}

func TestGovernance_StakeAmounts(t *testing.T) {
	var (
		lo = uint64(4000000) // lower then minStake
		ms = uint64(5000000) // minStake
		hi = uint64(6000000) // higher than minStake
	)

	configItems := makeSnapshotTestConfigItems()
	// because SetTestStakingManagerWithStakingInfoCache() sets only one cache entry,
	// we make sure all GetStakingInfo hits the only cache entry.
	configItems = append(configItems, stakingUpdateInterval(100))
	configItems = append(configItems, proposerUpdateInterval(1))
	configItems = append(configItems, proposerPolicy(params.WeightedRandom))
	configItems = append(configItems, minimumStake(new(big.Int).SetUint64(ms)))
	configItems = append(configItems, blockPeriod(0)) // set block period to 0 to prevent creating future block
	var (
		configItemsI  = append(configItems, istanbulCompatibleBlock(common.Big0))
		configItemsIS = append(configItemsI, governanceMode("single"))
	)

	testcases := []struct {
		configOverride []interface{}
		stakeAmounts   []uint64
		validators     []int
		demoted        []int
	}{
		// Before istanbul fork, no demoted regardless of stake amounts
		// See `if IsIstanbul {..}` in validator/weighted.go.
		{configItems, []uint64{lo, lo, lo, lo}, []int{0, 1, 2, 3}, []int{}},
		{configItems, []uint64{lo, lo, lo, hi}, []int{0, 1, 2, 3}, []int{}},
		{configItems, []uint64{lo, lo, hi, hi}, []int{0, 1, 2, 3}, []int{}},
		{configItems, []uint64{lo, hi, hi, hi}, []int{0, 1, 2, 3}, []int{}},
		{configItems, []uint64{hi, hi, hi, hi}, []int{0, 1, 2, 3}, []int{}},

		// After istanbul fork, demoted according to stake amounts
		// if nobody staked enough, everyone is allowed.
		{configItemsI, []uint64{lo, lo, lo, lo}, []int{0, 1, 2, 3}, []int{}},
		// if someone staked enough, only they are allowed.
		{configItemsI, []uint64{lo, lo, lo, hi}, []int{3}, []int{0, 1, 2}},
		{configItemsI, []uint64{lo, lo, hi, hi}, []int{2, 3}, []int{0, 1}},
		{configItemsI, []uint64{lo, hi, hi, hi}, []int{1, 2, 3}, []int{0}},
		{configItemsI, []uint64{hi, hi, hi, hi}, []int{0, 1, 2, 3}, []int{}},
		// the condition is (amount) >= (minStake)
		{configItemsI, []uint64{ms + 1, ms + 0, ms - 1, 0}, []int{0, 1}, []int{2, 3}},

		// Under "single" governance mode, the governing node is always validator
		// even if its staking amount is low.
		{configItemsIS, []uint64{lo, lo, lo, lo}, []int{0, 1, 2, 3}, []int{}},
		{configItemsIS, []uint64{lo, lo, lo, hi}, []int{0, 3}, []int{1, 2}},
		{configItemsIS, []uint64{lo, lo, hi, hi}, []int{0, 2, 3}, []int{1}},
		{configItemsIS, []uint64{lo, hi, hi, hi}, []int{0, 1, 2, 3}, []int{}},
		{configItemsIS, []uint64{hi, hi, hi, hi}, []int{0, 1, 2, 3}, []int{}},
	}

	for _, tc := range testcases {
		// Staking amounts are applied from block 2 because
		//   block 0 uses snapshot(0) = genesis.istanbulExtra = all nodes
		//   block 1 uses snapshot(0) = genesis.istanbulExtra = all nodes
		//   block 2 uses snapshot(1) = refreshed according to StakingInfo
		vtc := &votingTestCase{
			configOverride: tc.configOverride,
			stakeAmounts:   tc.stakeAmounts,
			endBlock:       1,
			expectedValset: map[int]testExpectedValset{
				0: {[]int{0, 1, 2, 3}, []int{}},
				1: {[]int{0, 1, 2, 3}, []int{}},
				2: {tc.validators, tc.demoted},
			},
		}
		suite.Run(t, newVotingTestSuite(vtc))
	}
}

func TestGovernance_ChangeMinStake(t *testing.T) {
	var (
		a40, a50, a60, a70, a80 uint64 = 4000000, 5000000, 6000000, 7000000, 8000000
		s45, s55, s65, s75, s85 string = "4500000", "5500000", "6500000", "7500000", "8500000"

		minStake = a40
	)
	_, _, _, _, _ = a40, a50, a60, a70, a80
	_, _, _, _, _ = s45, s55, s65, s75, s85

	configItems := makeSnapshotTestConfigItems()
	// because SetTestStakingManagerWithStakingInfoCache() sets only one cache entry,
	// we make sure all GetStakingInfo hits the only cache entry.
	configItems = append(configItems, stakingUpdateInterval(100))
	configItems = append(configItems, proposerUpdateInterval(1))
	configItems = append(configItems, proposerPolicy(params.WeightedRandom))
	configItems = append(configItems, minimumStake(new(big.Int).SetUint64(minStake)))
	configItems = append(configItems, epoch(3))
	configItems = append(configItems, blockPeriod(0)) // set block period to 0 to prevent creating future block
	var (
		configItemsI  = append(configItems, istanbulCompatibleBlock(common.Big0))
		configItemsIS = append(configItemsI, governanceMode("single"))
	)

	// ValSet update timing
	// - Vote "reward.minimumstake" at some block
	// - Vote applied at block #N
	// - Proposer updated at block #N+1, based on ParamsAt(N).
	testcases := []votingTestCase{
		{
			configItemsI, // "none" governance mode
			[]uint64{a80, a70, a60, a50},
			21,
			map[int]testVotes{
				1:  {"reward.minimumstake": s85}, // effects valset #7-
				4:  {"reward.minimumstake": s75}, // effects valset #10-
				7:  {"reward.minimumstake": s65}, // effects valset #13-
				10: {"reward.minimumstake": s55}, // effects valset #16-
				13: {"reward.minimumstake": s45}, // effects valset #19-
			},
			nil,
			map[int]testExpectedValset{
				// min = 40. above = {0,1,2,3}
				0: {[]int{0, 1, 2, 3}, []int{}},
				1: {[]int{0, 1, 2, 3}, []int{}},
				5: {[]int{0, 1, 2, 3}, []int{}},
				6: {[]int{0, 1, 2, 3}, []int{}},
				// min = 85. above = {}
				7: {[]int{0, 1, 2, 3}, []int{}},
				8: {[]int{0, 1, 2, 3}, []int{}},
				9: {[]int{0, 1, 2, 3}, []int{}},
				// min = 75. above = {0}
				10: {[]int{0}, []int{1, 2, 3}},
				11: {[]int{0}, []int{1, 2, 3}},
				12: {[]int{0}, []int{1, 2, 3}},
				// min = 65. above = {0,1}
				13: {[]int{0, 1}, []int{2, 3}},
				14: {[]int{0, 1}, []int{2, 3}},
				15: {[]int{0, 1}, []int{2, 3}},
				// min = 55. above = {0,1,2}
				16: {[]int{0, 1, 2}, []int{3}},
				17: {[]int{0, 1, 2}, []int{3}},
				18: {[]int{0, 1, 2}, []int{3}},
				// min = 45. above = {0,1,2,3}
				19: {[]int{0, 1, 2, 3}, []int{}},
				20: {[]int{0, 1, 2, 3}, []int{}},
				21: {[]int{0, 1, 2, 3}, []int{}},
			},
		},
		// Under "single" governance mode, the governing node is always validator
		// even if its staking amount is low.
		{
			configItemsIS, // "single" governance mode
			[]uint64{a50, a60, a70, a80},
			21,
			map[int]testVotes{
				1:  {"reward.minimumstake": s85}, // effects valset #7-
				4:  {"reward.minimumstake": s75}, // effects valset #10-
				7:  {"reward.minimumstake": s65}, // effects valset #13-
				10: {"reward.minimumstake": s55}, // effects valset #16-
				13: {"reward.minimumstake": s45}, // effects valset #19-
			},
			nil,
			map[int]testExpectedValset{
				// min = 40. above = {0,1,2,3}
				0: {[]int{0, 1, 2, 3}, []int{}},
				1: {[]int{0, 1, 2, 3}, []int{}},
				5: {[]int{0, 1, 2, 3}, []int{}},
				6: {[]int{0, 1, 2, 3}, []int{}},
				// min = 85. above = {}
				7: {[]int{0, 1, 2, 3}, []int{}},
				8: {[]int{0, 1, 2, 3}, []int{}},
				9: {[]int{0, 1, 2, 3}, []int{}},
				// min = 75. above = {3}
				10: {[]int{0, 3}, []int{1, 2}},
				11: {[]int{0, 3}, []int{1, 2}},
				12: {[]int{0, 3}, []int{1, 2}},
				// min = 65. above = {2,3}
				13: {[]int{0, 2, 3}, []int{1}},
				14: {[]int{0, 2, 3}, []int{1}},
				15: {[]int{0, 2, 3}, []int{1}},
				// min = 55. above = {1,2,3}
				16: {[]int{0, 1, 2, 3}, []int{}},
				17: {[]int{0, 1, 2, 3}, []int{}},
				18: {[]int{0, 1, 2, 3}, []int{}},
				// min = 45. above = {0,1,2,3}
				19: {[]int{0, 1, 2, 3}, []int{}},
				20: {[]int{0, 1, 2, 3}, []int{}},
				21: {[]int{0, 1, 2, 3}, []int{}},
			},
		},
		// Test "single" governance mode with governingNode change
		{
			configItemsIS, // "single" governance mode
			[]uint64{a60, a60, a50, a50},
			9,
			map[int]testVotes{
				1: {"reward.minimumstake": s55},    // effects valset #7-
				4: {"governance.governingnode": 2}, // effects valset #10-
			},
			nil,
			map[int]testExpectedValset{
				// min = 40. above = {0,1,2,3}. govNode = [0]
				0: {[]int{0, 1, 2, 3}, []int{}},
				1: {[]int{0, 1, 2, 3}, []int{}},
				5: {[]int{0, 1, 2, 3}, []int{}},
				6: {[]int{0, 1, 2, 3}, []int{}},
				// min = 55. above = {0,1}. govNode = [0]
				7: {[]int{0, 1}, []int{2, 3}},
				8: {[]int{0, 1}, []int{2, 3}},
				9: {[]int{0, 1}, []int{2, 3}},
				// min = 55. above = {0,1}. govNode = [2]
				10: {[]int{0, 1, 2}, []int{3}},
				11: {[]int{0, 1, 2}, []int{3}},
				12: {[]int{0, 1, 2}, []int{3}},
			},
		},
	}

	for _, tc := range testcases {
		suite.Run(t, newVotingTestSuite(&tc))
	}
}
