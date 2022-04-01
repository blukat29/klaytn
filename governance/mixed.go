package governance

import (
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/consensus/istanbul"
	"github.com/klaytn/klaytn/params"
	"github.com/klaytn/klaytn/storage/database"
)

type MixedEngine struct {
	initialConfig *params.ChainConfig

	// Data sources
	db database.DBManager

	// Subordinate engines
	defaultGov *Governance
}

func newMixedEngine(config *params.ChainConfig, db database.DBManager, doInit bool) *MixedEngine {
	e := &MixedEngine{
		initialConfig: config,
		db:            db,
	}

	if doInit {
		e.defaultGov = NewGovernanceInitialize(config, db)
	} else {
		e.defaultGov = NewGovernance(config, db)
	}
	return e
}

// Most users are encouraged to call this constructor
func NewMixedEngine(config *params.ChainConfig, db database.DBManager) *MixedEngine {
	return newMixedEngine(config, db, true)
}

// For test purposes
func NewMixedEngineNoInit(config *params.ChainConfig, db database.DBManager) *MixedEngine {
	return newMixedEngine(config, db, false)
}

// Pass-through to HeaderEngine
func (e *MixedEngine) AddVote(key string, val interface{}) bool {
	return e.defaultGov.AddVote(key, val)
}
func (e *MixedEngine) ValidateVote(vote *GovernanceVote) (*GovernanceVote, bool) {
	return e.defaultGov.ValidateVote(vote)
}
func (e *MixedEngine) CanWriteGovernanceState(num uint64) bool {
	return e.defaultGov.CanWriteGovernanceState(num)
}
func (e *MixedEngine) WriteGovernanceState(num uint64, isCheckpoint bool) error {
	return e.defaultGov.WriteGovernanceState(num, isCheckpoint)
}
func (e *MixedEngine) ReadGovernance(num uint64) (uint64, map[string]interface{}, error) {
	return e.defaultGov.ReadGovernance(num)
}
func (e *MixedEngine) WriteGovernance(num uint64, data GovernanceSet, delta GovernanceSet) error {
	return e.defaultGov.WriteGovernance(num, data, delta)
}
func (e *MixedEngine) GetEncodedVote(addr common.Address, number uint64) []byte {
	return e.defaultGov.GetEncodedVote(addr, number)
}
func (e *MixedEngine) GetGovernanceChange() map[string]interface{} {
	return e.defaultGov.GetGovernanceChange()
}
func (e *MixedEngine) VerifyGovernance(received []byte) error {
	return e.defaultGov.VerifyGovernance(received)
}
func (e *MixedEngine) ClearVotes(num uint64) {
	e.defaultGov.ClearVotes(num)
}
func (e *MixedEngine) WriteGovernanceForNextEpoch(number uint64, governance []byte) {
	e.defaultGov.WriteGovernanceForNextEpoch(number, governance)
}
func (e *MixedEngine) UpdateCurrentSet(num uint64) {
	e.defaultGov.UpdateCurrentSet(num)
}
func (e *MixedEngine) HandleGovernanceVote(
	valset istanbul.ValidatorSet, votes []GovernanceVote, tally []GovernanceTallyItem,
	header *types.Header, proposer common.Address, self common.Address) (
	istanbul.ValidatorSet, []GovernanceVote, []GovernanceTallyItem) {
	return e.defaultGov.HandleGovernanceVote(valset, votes, tally, header, proposer, self)
}
func (e *MixedEngine) ChainId() uint64 {
	return e.defaultGov.ChainId()
}
func (e *MixedEngine) InitialChainConfig() *params.ChainConfig {
	return e.defaultGov.InitialChainConfig()
}
func (e *MixedEngine) GetVoteMap() map[string]VoteStatus {
	return e.defaultGov.GetVoteMap()
}
func (e *MixedEngine) GetGovernanceTallies() []GovernanceTallyItem {
	return e.defaultGov.GetGovernanceTallies()
}
func (e *MixedEngine) PendingChanges() map[string]interface{} {
	return e.defaultGov.PendingChanges()
}
func (e *MixedEngine) Votes() []GovernanceVote {
	return e.defaultGov.Votes()
}
func (e *MixedEngine) IdxCache() []uint64 {
	return e.defaultGov.IdxCache()
}
func (e *MixedEngine) IdxCacheFromDb() []uint64 {
	return e.defaultGov.IdxCacheFromDb()
}
func (e *MixedEngine) NodeAddress() common.Address {
	return e.defaultGov.NodeAddress()
}
func (e *MixedEngine) TotalVotingPower() uint64 {
	return e.defaultGov.TotalVotingPower()
}
func (e *MixedEngine) MyVotingPower() uint64 {
	return e.defaultGov.MyVotingPower()
}
func (e *MixedEngine) BlockChain() blockChain {
	return e.defaultGov.BlockChain()
}
func (e *MixedEngine) DB() database.DBManager {
	return e.defaultGov.DB()
}
func (e *MixedEngine) SetNodeAddress(addr common.Address) {
	e.defaultGov.SetNodeAddress(addr)
}
func (e *MixedEngine) SetTotalVotingPower(t uint64) {
	e.defaultGov.SetTotalVotingPower(t)
}
func (e *MixedEngine) SetMyVotingPower(t uint64) {
	e.defaultGov.SetMyVotingPower(t)
}
func (e *MixedEngine) SetBlockchain(chain blockChain) {
	e.defaultGov.SetBlockchain(chain)
}
func (e *MixedEngine) SetTxPool(txpool txPool) {
	e.defaultGov.SetTxPool(txpool)
}
func (e *MixedEngine) GovernanceMode() string {
	return e.defaultGov.GovernanceMode()
}
func (e *MixedEngine) GoverningNode() common.Address {
	return e.defaultGov.GoverningNode()
}
func (e *MixedEngine) UnitPrice() uint64 {
	return e.defaultGov.UnitPrice()
}
func (e *MixedEngine) CommitteeSize() uint64 {
	return e.defaultGov.CommitteeSize()
}
func (e *MixedEngine) Epoch() uint64 {
	return e.defaultGov.Epoch()
}
func (e *MixedEngine) ProposerPolicy() uint64 {
	return e.defaultGov.ProposerPolicy()
}
func (e *MixedEngine) DeferredTxFee() bool {
	return e.defaultGov.DeferredTxFee()
}
func (e *MixedEngine) MinimumStake() string {
	return e.defaultGov.MinimumStake()
}
func (e *MixedEngine) MintingAmount() string {
	return e.defaultGov.MintingAmount()
}
func (e *MixedEngine) ProposerUpdateInterval() uint64 {
	return e.defaultGov.ProposerUpdateInterval()
}
func (e *MixedEngine) Ratio() string {
	return e.defaultGov.Ratio()
}
func (e *MixedEngine) StakingUpdateInterval() uint64 {
	return e.defaultGov.StakingUpdateInterval()
}
func (e *MixedEngine) UseGiniCoeff() bool {
	return e.defaultGov.UseGiniCoeff()
}
func (e *MixedEngine) GetGovernanceValue(key int) interface{} {
	return e.defaultGov.GetGovernanceValue(key)
}
func (e *MixedEngine) GetGovernanceItemAtNumber(num uint64, key string) (interface{}, error) {
	return e.defaultGov.GetGovernanceItemAtNumber(num, key)
}
func (e *MixedEngine) GetItemAtNumberByIntKey(num uint64, key int) (interface{}, error) {
	return e.defaultGov.GetItemAtNumberByIntKey(num, key)
}
func (e *MixedEngine) GetGoverningInfoAtNumber(num uint64) (bool, common.Address, error) {
	return e.defaultGov.GetGoverningInfoAtNumber(num)
}
func (e *MixedEngine) GetMinimumStakingAtNumber(num uint64) (uint64, error) {
	return e.defaultGov.GetMinimumStakingAtNumber(num)
}
func (e *MixedEngine) AddGovernanceCacheForTest(num uint64, config *params.ChainConfig) {
	e.defaultGov.AddGovernanceCacheForTest(num, config)
}
