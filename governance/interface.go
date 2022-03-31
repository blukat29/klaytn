// Copyright 2022 The klaytn Authors
// This file is part of the klaytn library.
//
// The klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the klaytn library. If not, see <http://www.gnu.org/licenses/>.

package governance

import (
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/consensus/istanbul"
)

type Engine interface {
	HeaderEngine
}

type HeaderEngine interface {
	// Cast votes from API
	AddVote(key string, val interface{}) bool
	ValidateVote(vote *GovernanceVote) (*GovernanceVote, bool)

	// Access database for voting states
	CanWriteGovernanceState(num uint64) bool
	WriteGovernanceState(num uint64, isCheckpoint bool) error

	// Access database for network params
	ReadGovernance(num uint64) (uint64, map[string]interface{}, error)
	WriteGovernance(num uint64, data GovernanceSet, delta GovernanceSet) error

	// Compose header.Vote and header.Governance
	GetEncodedVote(addr common.Address, number uint64) []byte
	GetGovernanceChange() map[string]interface{}

	// Intake header.Vote and header.Governance
	VerifyGovernance(received []byte) error
	ClearVotes(num uint64)
	WriteGovernanceForNextEpoch(number uint64, governance []byte)
	UpdateCurrentSet(num uint64)
	HandleGovernanceVote(
		valset istanbul.ValidatorSet, votes []GovernanceVote, tally []GovernanceTallyItem,
		header *types.Header, proposer common.Address, self common.Address) (
		istanbul.ValidatorSet, []GovernanceVote, []GovernanceTallyItem)

	// Get internal fields
	ChainId() uint64
	PendingChanges() map[string]interface{}
	Votes() []GovernanceVote
	IdxCache() []uint64
	IdxCacheFromDb() []uint64

	// Set internal fields
	SetNodeAddress(addr common.Address)
	SetTotalVotingPower(t uint64)
	SetMyVotingPower(t uint64)
	SetBlockchain(chain blockChain)
	SetTxPool(txpool txPool)

	// Get network params
	GovernanceMode() string
	GoverningNode() common.Address
	UnitPrice() uint64
	CommitteeSize() uint64
	Epoch() uint64
	ProposerPolicy() uint64
	DeferredTxFee() bool
	MinimumStake() string
	MintingAmount() string
	ProposerUpdateInterval() uint64
	Ratio() string
	StakingUpdateInterval() uint64
	UseGiniCoeff() bool
	GetGovernanceValue(key int) interface{}
	GetGovernanceItemAtNumber(num uint64, key string) (interface{}, error)
	GetItemAtNumberByIntKey(num uint64, key int) (interface{}, error)
	GetGoverningInfoAtNumber(num uint64) (bool, common.Address, error)
	GetMinimumStakingAtNumber(num uint64) (uint64, error)
}
