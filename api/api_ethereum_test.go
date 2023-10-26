package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klaytn/klaytn/consensus"
	"github.com/klaytn/klaytn/governance"

	"github.com/golang/mock/gomock"
	"github.com/klaytn/klaytn/accounts"
	mock_accounts "github.com/klaytn/klaytn/accounts/mocks"
	mock_api "github.com/klaytn/klaytn/api/mocks"
	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/blockchain/state"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/blockchain/types/accountkey"
	"github.com/klaytn/klaytn/blockchain/vm"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/common/hexutil"
	"github.com/klaytn/klaytn/consensus/gxhash"
	mock_consensus "github.com/klaytn/klaytn/consensus/mocks"
	"github.com/klaytn/klaytn/networks/rpc"
	"github.com/klaytn/klaytn/params"
	"github.com/klaytn/klaytn/rlp"
	"github.com/klaytn/klaytn/storage/database"
	"github.com/stretchr/testify/assert"
)

var (
	// Deprecated: in favor of testChainConfigEthCompat
	dummyChainConfigForEthereumAPITest = &params.ChainConfig{
		ChainID:                  new(big.Int).SetUint64(111111),
		IstanbulCompatibleBlock:  new(big.Int).SetUint64(0),
		LondonCompatibleBlock:    new(big.Int).SetUint64(0),
		EthTxTypeCompatibleBlock: new(big.Int).SetUint64(0),
		UnitPrice:                25000000000, // 25 ston
	}

	testLondonConfig = &params.ChainConfig{
		ChainID:                 new(big.Int).SetUint64(111111),
		IstanbulCompatibleBlock: common.Big0,
		LondonCompatibleBlock:   common.Big0,
		UnitPrice:               25000000000,
	}
	testEthTxTypeConfig = &params.ChainConfig{
		ChainID:                  new(big.Int).SetUint64(111111),
		IstanbulCompatibleBlock:  common.Big0,
		LondonCompatibleBlock:    common.Big0,
		EthTxTypeCompatibleBlock: common.Big0,
		UnitPrice:                25000000000, // 25 ston
	}
	testMagmaConfig = &params.ChainConfig{
		ChainID:                  new(big.Int).SetUint64(111111),
		IstanbulCompatibleBlock:  common.Big0,
		LondonCompatibleBlock:    common.Big0,
		EthTxTypeCompatibleBlock: common.Big0,
		MagmaCompatibleBlock:     common.Big0,
		UnitPrice:                25000000000, // 25 ston
	}
	testRandaoConfig = &params.ChainConfig{
		ChainID:                  new(big.Int).SetUint64(111111),
		IstanbulCompatibleBlock:  common.Big0,
		LondonCompatibleBlock:    common.Big0,
		EthTxTypeCompatibleBlock: common.Big0,
		MagmaCompatibleBlock:     common.Big0,
		KoreCompatibleBlock:      common.Big0,
		ShanghaiCompatibleBlock:  common.Big0,
		CancunCompatibleBlock:    common.Big0,
		RandaoCompatibleBlock:    common.Big0,
		UnitPrice:                25000000000, // 25 ston
	}

	// To be filled by fillTestData()
	testChainConfig *params.ChainConfig
	testNodeAddr    common.Address
	testAuthor      common.Address
	testTd          *big.Int

	testHeader       *types.Header
	testHeaderMap    map[string]interface{} // expected getHeader(num)
	testBlock        *types.Block
	testBlockMapFull map[string]interface{} // expected getBlock(num, true)
	testBlockMapHash map[string]interface{} // expected getBlock(num, false)

	testTxObjects        []interface{}
	testTxObjectsPending []interface{} // tx object without block info (blockNum, blockHash, txIndex)
	testReceipts         []interface{}

	// TODO: make named error types
	testErrUnknownHeader error
	testErrUnknownBlock  error
)

func TestEthAPI(t *testing.T) {
	// Generate test data
	blockchain.InitDeriveSha(testLondonConfig)
	fillTestData(t, testLondonConfig)

	// param shortcuts
	var (
		bg      = context.Background()
		hash0   = common.Hash{}
		hashAny = common.HexToHash("0x1234")
		num0    = rpc.BlockNumber(0)
		numAny  = rpc.BlockNumber(4)
		u64_0   = hexutil.Uint64(0)
		uint0   = hexutil.Uint(0)
		uintNil = (*hexutil.Uint)(nil)
		mapNil  = (map[string]interface{})(nil)
	)

	// Simple constant values
	testEthApiCall(t, nil, u64_0, "Hashrate")
	testEthApiCall(t, nil, false, "Mining")
	testEthApiFail(t, nil, errNoMiningWork, "GetWork")
	testEthApiCall(t, nil, false, "SubmitWork", BlockNonce{}, hash0, hash0)
	testEthApiCall(t, nil, false, "SubmitHashrate", u64_0, hash0)
	testEthApiCall(t, nil, ZeroHashrate, "GetHashrate")
	testEthApiCall(t, nil, map[string]interface{}(nil), "GetUncleByBlockNumberAndIndex", bg, num0, uint0)
	testEthApiCall(t, nil, map[string]interface{}(nil), "GetUncleByBlockHashAndIndex", bg, hash0, uint0)

	// Governance NodeAddress
	testEthApiCall(t, []mockFunc{mockNodeAddress}, testNodeAddr, "Etherbase")
	testEthApiCall(t, []mockFunc{mockNodeAddress}, testNodeAddr, "Coinbase")

	// Uncle count
	testEthApiCall(t, []mockFunc{mockBlock}, &uint0, "GetUncleCountByBlockNumber", bg, numAny)
	testEthApiCall(t, []mockFunc{mockBlock}, &uint0, "GetUncleCountByBlockHash", bg, hashAny)
	testEthApiCall(t, []mockFunc{mockNoBlock}, uintNil, "GetUncleCountByBlockNumber", bg, numAny)
	testEthApiCall(t, []mockFunc{mockNoBlock}, uintNil, "GetUncleCountByBlockHash", bg, hashAny)

	// Headers and blocks, not found cases. Returns null instaed of error.
	testEthApiCall(t, []mockFunc{mockNoBlock}, mapNil, "GetHeaderByNumber", bg, numAny)
	testEthApiCall(t, []mockFunc{mockNoBlock}, mapNil, "GetHeaderByHash", bg, hashAny)
	testEthApiCall(t, []mockFunc{mockNoBlock}, mapNil, "GetBlockByNumber", bg, numAny, false)
	testEthApiCall(t, []mockFunc{mockNoBlock}, mapNil, "GetBlockByHash", bg, hashAny, false)

	// Below APIs show different behaviors according to hardfork status
	configs := []*params.ChainConfig{
		testLondonConfig,
		testEthTxTypeConfig,
		testMagmaConfig,
		testRandaoConfig,
	}
	for _, config := range configs {
		fillTestData(t, config)
		// headers and blocks
		testEthApiJson(t, []mockFunc{mockBlock}, testHeaderMap, "GetHeaderByNumber", bg, numAny)
		testEthApiJson(t, []mockFunc{mockBlock}, testHeaderMap, "GetHeaderByHash", bg, hashAny)
		testEthApiJson(t, []mockFunc{mockBlock}, testBlockMapHash, "GetBlockByNumber", bg, numAny, false)
		testEthApiJson(t, []mockFunc{mockBlock}, testBlockMapFull, "GetBlockByNumber", bg, numAny, true)
		testEthApiJson(t, []mockFunc{mockBlock}, testBlockMapHash, "GetBlockByHash", bg, hashAny, false)
		testEthApiJson(t, []mockFunc{mockBlock}, testBlockMapFull, "GetBlockByHash", bg, hashAny, true)
	}

	// Transactions and receipts
	testEthApiForTxIndex(t, []mockFunc{mockBlock}, testTxObjects, "GetTransactionByBlockNumberAndIndex", bg, numAny)
	testEthApiForTxIndex(t, []mockFunc{mockBlock}, testTxObjects, "GetTransactionByBlockHashAndIndex", bg, hashAny)
	testEthApiForTxHash(t, []mockFunc{mockBlock, mockChainDBTx}, testTxObjects,
		"GetTransactionByHash", bg)
	testEthApiForTxHash(t, []mockFunc{mockBlock, mockChainDBNoTx, mockPoolTx}, testTxObjectsPending,
		"GetTransactionByHash", bg)
	t.Run("PendingTransactions", testEthereumAPI_PendingTransactions)
	testEthApiForTxHash(t, []mockFunc{mockBlock, mockReceipt}, testReceipts, "GetTransactionReceipt", bg)
}

func testEthApiFail(t *testing.T, injects []mockFunc, expected error, method string, params ...interface{}) {
	t.Run(method, func(t *testing.T) {
		_, err := mockCallEthApi(t, injects, method, params...)
		assert.ErrorIs(t, err, expected)
	})
}

func testEthApiCall(t *testing.T, injects []mockFunc, expected interface{}, method string, params ...interface{}) {
	t.Run(method, func(t *testing.T) {
		result, err := mockCallEthApi(t, injects, method, params...)
		assert.Nil(t, err)
		assert.Equal(t, expected, result)
	})
}

func testEthApiJson(t *testing.T, injects []mockFunc, expected interface{}, method string, params ...interface{}) {
	t.Run(method, func(t *testing.T) {
		result, err := mockCallEthApi(t, injects, method, params...)
		assert.Nil(t, err)

		resultMap := stringifyResult(t, result)
		assert.Equal(t, expected, resultMap)
	})
}

// Test the call 'method(params..., idx)' for each txs in testTxObjectsJson
func testEthApiForTxIndex(t *testing.T, injects []mockFunc, expectedList []interface{}, method string, params ...interface{}) {
	assert.NotEmpty(t, expectedList)
	t.Run(method, func(t *testing.T) {
		for idx, expected := range expectedList {
			paramsIdx := append(params, hexutil.Uint(idx))

			result, err := mockCallEthApi(t, injects, method, paramsIdx...)
			assert.Nil(t, err)

			resultMap := stringifyResult(t, result)
			assert.Equal(t, expected, resultMap)
		}
	})
}

func testEthApiForTxHash(t *testing.T, injects []mockFunc, expectedList []interface{}, method string, params ...interface{}) {
	assert.NotEmpty(t, expectedList)
	t.Run(method, func(t *testing.T) {
		for _, expected := range expectedList {
			expectedMap := expected.(map[string]interface{})
			var txhash string
			if v, ok := expectedMap["hash"]; ok {
				txhash = v.(string)
			} else if v, ok := expectedMap["transactionHash"]; ok {
				txhash = v.(string)
			}
			paramsHash := append(params, common.HexToHash(txhash))

			result, err := mockCallEthApi(t, injects, method, paramsHash...)
			assert.Nil(t, err)

			resultMap := stringifyResult(t, result)
			assert.Equal(t, expected, resultMap)
			break
		}
	})
}

func stringifyResult(t *testing.T, result interface{}) map[string]interface{} {
	resultJsonBytes, err := json.Marshal(result)
	assert.Nil(t, err)
	var resultMap map[string]interface{}
	err = json.Unmarshal(resultJsonBytes, &resultMap)
	assert.Nil(t, err)
	return resultMap
}

// testContext is a context for testing EthereumAPI with mock backend.
// Stuff the mock backend using mockInjectFunc.
type testContext struct {
	mockCtrl    *gomock.Controller
	mockBackend *mock_api.MockBackend
	api         *EthereumAPI
}
type mockFunc func(ctx *testContext)

func mockCallEthApi(t *testing.T, injects []mockFunc, method string, params ...interface{}) (interface{}, error) {
	ctx := &testContext{}

	// Setup mock backend and API
	if len(injects) == 0 {
		ctx.api = &EthereumAPI{}
	} else {
		ctx.mockCtrl = gomock.NewController(t)
		ctx.mockBackend = mock_api.NewMockBackend(ctx.mockCtrl)
		ctx.api = &EthereumAPI{
			publicTransactionPoolAPI: NewPublicTransactionPoolAPI(ctx.mockBackend, new(AddrLocker)),
			publicKlayAPI:            NewPublicKlayAPI(ctx.mockBackend),
			publicBlockChainAPI:      NewPublicBlockChainAPI(ctx.mockBackend),
		}
		defer ctx.mockCtrl.Finish()

		for _, inject := range injects {
			inject(ctx)
		}
	}

	// Invoke the method from generic params
	api := ctx.api
	values := make([]reflect.Value, len(params))
	for i, param := range params {
		values[i] = reflect.ValueOf(param)
	}
	methodPtr := reflect.ValueOf(api).MethodByName(method)
	results := methodPtr.Call(values)

	// Assume that the method returns [] or [result] or [result, err]
	var result interface{}
	if len(results) > 0 {
		result = results[0].Interface()
	}
	var err error
	if len(results) > 1 && results[1].Interface() != nil {
		err = results[1].Interface().(error)
	}
	return result, err
}

// Below are mockInjectFuncs for various backend features.

// Stuff gov.NodeAddress()
func mockNodeAddress(ctx *testContext) {
	var (
		dbm    = database.NewMemoryDBManager()
		gov    = governance.NewMixedEngineNoInit(testLondonConfig, dbm)
		govAPI = governance.NewGovernanceAPI(gov)
	)
	gov.SetNodeAddress(testNodeAddr)

	ctx.api.governanceAPI = govAPI
}

// Stuff backend.BlockByNumber() and backend.BlockByHash() with valid block
func mockBlock(ctx *testContext) {
	mockEngine := mock_consensus.NewMockEngine(ctx.mockCtrl)
	mockEngine.EXPECT().Author(gomock.Any()).Return(testAuthor, nil).AnyTimes()

	ctx.mockBackend.EXPECT().Engine().Return(mockEngine).AnyTimes()
	ctx.mockBackend.EXPECT().GetTd(gomock.Any()).Return(testTd).AnyTimes()
	ctx.mockBackend.EXPECT().ChainConfig().Return(testChainConfig).AnyTimes()

	ctx.mockBackend.EXPECT().HeaderByNumber(gomock.Any(), gomock.Any()).Return(testHeader, nil).AnyTimes()
	ctx.mockBackend.EXPECT().HeaderByHash(gomock.Any(), gomock.Any()).Return(testHeader, nil).AnyTimes()

	ctx.mockBackend.EXPECT().BlockByNumber(gomock.Any(), gomock.Any()).Return(testBlock, nil).AnyTimes()
	ctx.mockBackend.EXPECT().BlockByHash(gomock.Any(), gomock.Any()).Return(testBlock, nil).AnyTimes()
}

// Stuff backend.BlockByNumber() and backend.BlockByHash() with unknown block error
func mockNoBlock(ctx *testContext) {
	ctx.mockBackend.EXPECT().HeaderByNumber(gomock.Any(), gomock.Any()).Return(nil, testErrUnknownHeader).AnyTimes()
	ctx.mockBackend.EXPECT().HeaderByHash(gomock.Any(), gomock.Any()).Return(nil, testErrUnknownHeader).AnyTimes()

	ctx.mockBackend.EXPECT().BlockByNumber(gomock.Any(), gomock.Any()).Return(nil, testErrUnknownBlock).AnyTimes()
	ctx.mockBackend.EXPECT().BlockByHash(gomock.Any(), gomock.Any()).Return(nil, testErrUnknownBlock).AnyTimes()
}

// Stuff txpool.ChainDB().ReadTxAndLookupInfo()
func mockChainDBTx(ctx *testContext) {
	txs := make(map[common.Hash]*types.Transaction)
	for _, tx := range testBlock.Transactions() {
		txs[tx.Hash()] = tx
	}

	mockDBManager := &mockChainDB{txs: txs, block: testBlock}
	ctx.mockBackend.EXPECT().ChainDB().Return(mockDBManager).AnyTimes()
}

// Stuff backend.ChainDB().ReadTxAndLookupInfo() with no data
func mockChainDBNoTx(ctx *testContext) {
	txs := make(map[common.Hash]*types.Transaction)
	mockDBManager := &mockChainDB{txs: txs, block: testBlock}
	ctx.mockBackend.EXPECT().ChainDB().Return(mockDBManager).AnyTimes()
}

// Stuff backend.GetPoolTransaction()
func mockPoolTx(ctx *testContext) {
	txs := make(map[common.Hash]*types.Transaction)
	for _, tx := range testBlock.Transactions() {
		txs[tx.Hash()] = tx
	}

	ctx.mockBackend.EXPECT().GetPoolTransaction(gomock.Any()).DoAndReturn(
		func(hash common.Hash) *types.Transaction {
			return txs[hash]
		}).AnyTimes()
}

// Stuff backend.GetTxLookupInfoAndReceipt(), backend.GetBlockReceipts()
func mockReceipt(ctx *testContext) {
	txs := make(map[common.Hash]*types.Transaction)
	receiptList := make([]*types.Receipt, 0)
	receiptMap := make(map[common.Hash]*types.Receipt)
	for _, tx := range testBlock.Transactions() {
		txs[tx.Hash()] = tx
		receipt := types.NewReceipt(0, tx.Hash(), tx.Gas())
		receiptList = append(receiptList, receipt)
		receiptMap[tx.Hash()] = receipt
	}

	ctx.mockBackend.EXPECT().GetTxLookupInfoAndReceipt(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, *types.Receipt) {
			return txs[hash], testBlock.Hash(), testBlock.NumberU64(), txs[hash].Nonce(), receiptMap[hash]
		}).AnyTimes()
	ctx.mockBackend.EXPECT().GetBlockReceipts(gomock.Any(), gomock.Any()).Return(receiptList).AnyTimes()
}

// mockChainDB is a mock of database.DBManager for overriding the backend.ChainDB().ReadTxAndLookupInfo().
type mockChainDB struct {
	database.DBManager

	txs   map[common.Hash]*types.Transaction
	block *types.Block
}

func (m *mockChainDB) ReadTxAndLookupInfo(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	tx := m.txs[hash]
	if tx == nil {
		return nil, common.Hash{}, 0, 0
	} else {
		return tx, m.block.Hash(), m.block.NumberU64(), tx.Nonce()
	}
}

func fillTestData(t *testing.T, config *params.ChainConfig) {
	rules := config.Rules(big.NewInt(4))

	testChainConfig = config
	testNodeAddr = common.HexToAddress("0x9712f943b296758aaae79944ec975884188d3a96")
	testAuthor = common.HexToAddress("0x2eaad2bf70a070aaa2e007beee99c6148f47718e")
	testTd = big.NewInt(5)

	testHeader, testHeaderMap = createTestHeader(rules)
	assert.Equal(t, testHeader.Hash().Hex(), testHeaderMap["hash"])

	testBlock, testBlockMapHash, testBlockMapFull = createTestBlock(t, rules, testHeader, testHeaderMap)

	testTxObjects = testBlockMapFull["transactions"].([]interface{})
	testTxObjectsPending = make([]interface{}, 0)
	for _, elem := range testTxObjects {
		txObj := cloneJsonMap(elem.(map[string]interface{}))
		txObj["blockNumber"] = nil
		txObj["blockHash"] = nil
		txObj["transactionIndex"] = nil
		testTxObjectsPending = append(testTxObjectsPending, txObj)
	}
	testReceipts = make([]interface{}, 0)
	for _, elem := range testTxObjects {
		receipt := cloneJsonMap(elem.(map[string]interface{}))
		receipt["status"] = "0x0"
		receipt["logsBloom"] = "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
		receipt["logs"] = make([]interface{}, 0)
		receipt["gasUsed"] = receipt["gas"]
		receipt["effectiveGasPrice"] = "0x5d21dba00"
		receipt["transactionHash"] = receipt["hash"]

		delete(receipt, "hash")
		testReceipts = append(testReceipts, receipt)
		fmt.Println("copying", receipt)
		break
	}

	testErrUnknownBlock = fmt.Errorf("the block does not exist (block number: %d)", 4)
	testErrUnknownHeader = fmt.Errorf("the header does not exist (block number: %d)", 4)
}

func cloneJsonMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{})
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func createTestHeader(rules params.Rules) (*types.Header, map[string]interface{}) {
	header := &types.Header{
		ParentHash:  common.HexToHash("0xc8036293065bacdfce87debec0094a71dbbe40345b078d21dcc47adb4513f348"),
		Rewardbase:  common.HexToAddress("0x9712f943b296758aaae79944ec975884188d3a96"),
		TxHash:      common.HexToHash("0x0a83e34ab7302f42f4a9203e8295f545517645989da6555d8cbdc1e9599df85b"),
		Root:        common.HexToHash("0xad31c32942fa033166e4ef588ab973dbe26657c594de4ba98192108becf0fec9"),
		ReceiptHash: common.HexToHash("0xf6278dd71ffc1637f78dc2ee54f6f9e64d4b1633c1179dfdbc8c3b482efbdbec"),
		Bloom:       types.BytesToBloom(hexutil.MustDecode("0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")),
		BlockScore:  big.NewInt(1),
		Number:      big.NewInt(4),
		GasUsed:     21000,
		Time:        big.NewInt(1641363540),
		TimeFoS:     85,
		Extra:       common.Hex2Bytes("0xd983010701846b6c617988676f312e31362e338664617277696e000000000000f89ed5949712f943b296758aaae79944ec975884188d3a96b8415a0614be7fd5ea40f11ce558e02993bd55f11ae72a3cfbc861875a57483ec5ec3adda3e5845fd7ab271d670c755480f9ef5b8dd731f4e1f032fff5d165b763ac01f843b8418867d3733167a0c737fa5b62dcc59ec3b0af5748bcc894e7990a0b5a642da4546713c9127b3358cdfe7894df1ca1db5a97560599986d7f1399003cd63660b98200"),
		Governance:  hexutil.MustDecode("0x9b7b227265776172642e726174696f223a2235302f32302f3330227d"),
		Vote:        hexutil.MustDecode("0xf09499fb17d324fa0e07f23b49d09028ac0919414db694676f7665726e616e63652e756e6974707269636585ae9f7bcc00"),
	}
	headerMap := map[string]interface{}{
		"difficulty":       "0x1",
		"extraData":        "0x",           // Dropped in eth_ namespace
		"gasLimit":         "0xe8d4a50fff", // UpperGasLimit
		"gasUsed":          "0x5208",
		"hash":             "0x9b9e7a3706eee41e1aa16357916633c8a3523fdf0daaae44380c4a07a22e6e9b",
		"logsBloom":        "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		"miner":            "0x2eaad2bf70a070aaa2e007beee99c6148f47718e",
		"mixHash":          "0x0000000000000000000000000000000000000000000000000000000000000000",
		"nonce":            "0x0000000000000000", // BlockNonce is fixed 8 byte
		"number":           "0x4",
		"parentHash":       "0xc8036293065bacdfce87debec0094a71dbbe40345b078d21dcc47adb4513f348",
		"receiptsRoot":     "0xf6278dd71ffc1637f78dc2ee54f6f9e64d4b1633c1179dfdbc8c3b482efbdbec",
		"sha3Uncles":       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"size":             "0x291",
		"stateRoot":        "0xad31c32942fa033166e4ef588ab973dbe26657c594de4ba98192108becf0fec9",
		"timestamp":        "0x61d53854",
		"totalDifficulty":  "0x5",
		"transactionsRoot": "0x0a83e34ab7302f42f4a9203e8295f545517645989da6555d8cbdc1e9599df85b",
	}

	// Add optional fields according to the hardfork status
	if rules.IsEthTxType {
		headerMap["baseFeePerGas"] = "0x0" // Before Magma, return dummy value even if header.BaseFee == nil
	}
	if rules.IsMagma {
		header.BaseFee = big.NewInt(25000000000)
		headerMap["hash"] = "0x55a95dfb0da4233c0db6df7f411719cc1c94e95db21139028d0da81e4a961768"
		headerMap["baseFeePerGas"] = "0x5d21dba00"
		headerMap["size"] = "0x295"
	}
	if rules.IsRandao {
		header.RandomReveal = hexutil.MustDecode("0x94516a8bc695b5bf43aa077cd682d9475a3a6bed39a633395b78ed8f276e7c5bb00bb26a77825013c6718579f1b3ee2275b158801705ea77989e3acc849ee9c524bd1822bde3cba7be2aae04347f0d91508b7b7ce2f11ec36cbf763173421ae7")
		header.MixHash = hexutil.MustDecode("0xdf117d1245dceaae0a47f05371b23cd0d0db963ff9d5c8ba768dc989f4c31883")
		headerMap["hash"] = "0x39921290e5a25bfb5007cd0ad84e02ebad2e76f4ab139cadc5e31ff07120906e"
		//headerMap["mixHash"] = "0xdf117d1245dceaae0a47f05371b23cd0d0db963ff9d5c8ba768dc989f4c31883" // TODO: fix
		headerMap["mixHash"] = "0x0000000000000000000000000000000000000000000000000000000000000000"
		headerMap["size"] = "0x315"
	}
	return header, headerMap
}

func createTestBlock(t *testing.T, rules params.Rules, header *types.Header, headerMap map[string]interface{},
) (*types.Block, map[string]interface{}, map[string]interface{}) {
	block, _, _, _, _ := createTestData(t, header)
	blockMap := cloneJsonMap(headerMap)

	// Adjust uncles field
	blockMap["uncles"] = []interface{}{}

	// Adjust size field
	blockMap["size"] = "0xe91"
	if rules.IsMagma {
		blockMap["size"] = "0xe97"
	}
	if rules.IsRandao {
		blockMap["size"] = "0xf1a"
	}

	// Add transactions field
	var txhashList []interface{}
	assert.Nil(t, json.Unmarshal([]byte(testTxHashesJson), &txhashList))
	blockMapHash := cloneJsonMap(blockMap)
	blockMapHash["transactions"] = txhashList

	var txObjectList []interface{}
	assert.Nil(t, json.Unmarshal([]byte(testTxObjectsJson), &txObjectList))
	for i := range txObjectList {
		txObjectList[i].(map[string]interface{})["blockHash"] = blockMap["hash"]
	}
	blockMapFull := cloneJsonMap(blockMap)
	blockMapFull["transactions"] = txObjectList

	return block, blockMapHash, blockMapFull
}

var testTxHashesJson = `[
	"0x6231f24f79d28bb5b8425ce577b3b77cd9c1ab766fcfc5233358a2b1c2f4ff70",
	"0xf146858415c060eae65a389cbeea8aeadc79461038fbee331ffd97b41279dd63",
	"0x0a01fc67bb4c15c32fa43563c0fcf05cd5bf2fdcd4ec78122b5d0295993bca24",
	"0x486f7561375c38f1627264f8676f92ec0dd1c4a7c52002ba8714e61fcc6bb649",
	"0xbd3e57cd31dd3d6679326f7a949f0de312e9ae53bec5ef3c23b43a5319c220a4",
	"0xff666129a0c7227b17681d668ecdef5d6681fc93dbd58856eea1374880c598b0",
	"0xa8ad4f295f2acff9ef56b476b1c52ecb74fb3fd95a789d768c2edb3376dbeacf",
	"0x47dbfd201fc1dd4188fd2003c6328a09bf49414be607867ca3a5d63573aede93",
	"0x2283294e89b41df2df4dd37c375a3f51c3ad11877aa0a4b59d0f68cf5cfd865a",
	"0x80e05750d02d22d73926179a0611b431ae7658846406f836e903d76191423716",
	"0xe8abdee5e8fef72fe4d98f7dbef36000407e97874e8c880df4d85646958dd2c1",
	"0x4c970be1815e58e6f69321202ce38b2e5c5e5ecb70205634848afdbc57224811",
	"0x7ff0a809387d0a4cab77624d467f4d65ffc1ac95f4cc46c2246daab0407a7d83",
	"0xb510b11415b39d18a972a00e3b43adae1e0f583ea0481a4296e169561ff4d916",
	"0xab466145fb71a2d24d6f6af3bddf3bcfa43c20a5937905dd01963eaf9fc5e382",
	"0xec714ab0875768f482daeabf7eb7be804e3c94bc1f1b687359da506c7f3a66b2",
	"0x069af125fe88784e46f90ace9960a09e5d23e6ace20350062be75964a7ece8e6",
	"0x4a6bb7b2cd68265eb6a693aa270daffa3cc297765267f92be293b12e64948c82",
	"0xa354fe3fdde6292e85545e6327c314827a20e0d7a1525398b38526fe28fd36e1",
	"0x5bb64e885f196f7b515e62e3b90496864d960e2f5e0d7ad88550fa1c875ca691",
	"0x6f4308b3c98db2db215d02c0df24472a215df7aa283261fcb06a6c9f796df9af",
	"0x1df88d113f0c5833c1f7264687cd6ac43888c232600ffba8d3a7d89bb5013e71"
]`

// TODO: Add TxTypeEthereumAccessList and TxTypeEthereumDynamicFee
var testTxObjectsJson = `[
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x0000000000000000000000000000000000000000",
		"gas": "0x1c9c380",
		"gasPrice": "0x5d21dba00",
		"hash": "0x6231f24f79d28bb5b8425ce577b3b77cd9c1ab766fcfc5233358a2b1c2f4ff70",
		"input": "0x3078653331393765386630303030303030303030303030303030303030303030303065306265663939623461323232383665323736333062343835643036633561313437636565393331303030303030303030303030303030303030303030303030313538626566663863386364656264363436353461646435663661316439393337653733353336633030303030303030303030303030303030303030303030303030303030303030303030303030303030303030323962623565376662366265616533326366383030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030306530303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303138303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030316236306662343631346132326530303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303031303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030343030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303031353862656666386338636465626436343635346164643566366131643939333765373335333663303030303030303030303030303030303030303030303030373462613033313938666564326231356135316166323432623963363366616633633866346433343030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303033303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030",
		"nonce": "0x0",
		"to": "0x3736346135356338333362313038373730343930",
		"transactionIndex": "0x0",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3036656164333031646165616636376537376538",
		"gas": "0x989680",
		"gasPrice": "0x5d21dba00",
		"hash": "0xf146858415c060eae65a389cbeea8aeadc79461038fbee331ffd97b41279dd63",
		"input": "0x",
		"nonce": "0x1",
		"to": "0x3364613566326466626334613262333837316462",
		"transactionIndex": "0x1",
		"value": "0x5",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3730323366383135666136613633663761613063",
		"gas": "0x1312d00",
		"gasPrice": "0x5d21dba00",
		"hash": "0x0a01fc67bb4c15c32fa43563c0fcf05cd5bf2fdcd4ec78122b5d0295993bca24",
		"input": "0x68656c6c6f",
		"nonce": "0x2",
		"to": "0x3336623562313539333066323466653862616538",
		"transactionIndex": "0x2",
		"value": "0x3",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x1312d00",
		"gasPrice": "0x5d21dba00",
		"hash": "0x486f7561375c38f1627264f8676f92ec0dd1c4a7c52002ba8714e61fcc6bb649",
		"input": "0x",
		"nonce": "0x3",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0x3",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x5f5e100",
		"gasPrice": "0x5d21dba00",
		"hash": "0xbd3e57cd31dd3d6679326f7a949f0de312e9ae53bec5ef3c23b43a5319c220a4",
		"input": "0x",
		"nonce": "0x4",
		"to": null,
		"transactionIndex": "0x4",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0xff666129a0c7227b17681d668ecdef5d6681fc93dbd58856eea1374880c598b0",
		"input": "0x",
		"nonce": "0x5",
		"to": "0x3632323232656162393565396564323963346266",
		"transactionIndex": "0x5",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0xa8ad4f295f2acff9ef56b476b1c52ecb74fb3fd95a789d768c2edb3376dbeacf",
		"input": "0x",
		"nonce": "0x6",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0x6",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0x47dbfd201fc1dd4188fd2003c6328a09bf49414be607867ca3a5d63573aede93",
		"input": "0xf8ad80b8aaf8a8a0072409b14b96f9d7dbf4788dbc68c5d30bd5fac1431c299e0ab55c92e70a28a4a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a00000000000000000000000000000000000000000000000000000000000000000a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a00000000000000000000000000000000000000000000000000000000000000000808080",
		"nonce": "0x7",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0x7",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3036656164333031646165616636376537376538",
		"gas": "0x989680",
		"gasPrice": "0x5d21dba00",
		"hash": "0x2283294e89b41df2df4dd37c375a3f51c3ad11877aa0a4b59d0f68cf5cfd865a",
		"input": "0x",
		"nonce": "0x8",
		"to": "0x3364613566326466626334613262333837316462",
		"transactionIndex": "0x8",
		"value": "0x5",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3730323366383135666136613633663761613063",
		"gas": "0x1312d00",
		"gasPrice": "0x5d21dba00",
		"hash": "0x80e05750d02d22d73926179a0611b431ae7658846406f836e903d76191423716",
		"input": "0x68656c6c6f",
		"nonce": "0x9",
		"to": "0x3336623562313539333066323466653862616538",
		"transactionIndex": "0x9",
		"value": "0x3",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x1312d00",
		"gasPrice": "0x5d21dba00",
		"hash": "0xe8abdee5e8fef72fe4d98f7dbef36000407e97874e8c880df4d85646958dd2c1",
		"input": "0x",
		"nonce": "0xa",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0xa",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x5f5e100",
		"gasPrice": "0x5d21dba00",
		"hash": "0x4c970be1815e58e6f69321202ce38b2e5c5e5ecb70205634848afdbc57224811",
		"input": "0x",
		"nonce": "0xb",
		"to": null,
		"transactionIndex": "0xb",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0x7ff0a809387d0a4cab77624d467f4d65ffc1ac95f4cc46c2246daab0407a7d83",
		"input": "0x",
		"nonce": "0xc",
		"to": "0x3632323232656162393565396564323963346266",
		"transactionIndex": "0xc",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0xb510b11415b39d18a972a00e3b43adae1e0f583ea0481a4296e169561ff4d916",
		"input": "0x",
		"nonce": "0xd",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0xd",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0xab466145fb71a2d24d6f6af3bddf3bcfa43c20a5937905dd01963eaf9fc5e382",
		"input": "0xf8ad80b8aaf8a8a0072409b14b96f9d7dbf4788dbc68c5d30bd5fac1431c299e0ab55c92e70a28a4a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a00000000000000000000000000000000000000000000000000000000000000000a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a00000000000000000000000000000000000000000000000000000000000000000808080",
		"nonce": "0xe",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0xe",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3036656164333031646165616636376537376538",
		"gas": "0x989680",
		"gasPrice": "0x5d21dba00",
		"hash": "0xec714ab0875768f482daeabf7eb7be804e3c94bc1f1b687359da506c7f3a66b2",
		"input": "0x",
		"nonce": "0xf",
		"to": "0x3364613566326466626334613262333837316462",
		"transactionIndex": "0xf",
		"value": "0x5",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3730323366383135666136613633663761613063",
		"gas": "0x1312d00",
		"gasPrice": "0x5d21dba00",
		"hash": "0x069af125fe88784e46f90ace9960a09e5d23e6ace20350062be75964a7ece8e6",
		"input": "0x68656c6c6f",
		"nonce": "0x10",
		"to": "0x3336623562313539333066323466653862616538",
		"transactionIndex": "0x10",
		"value": "0x3",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x1312d00",
		"gasPrice": "0x5d21dba00",
		"hash": "0x4a6bb7b2cd68265eb6a693aa270daffa3cc297765267f92be293b12e64948c82",
		"input": "0x",
		"nonce": "0x11",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0x11",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x5f5e100",
		"gasPrice": "0x5d21dba00",
		"hash": "0xa354fe3fdde6292e85545e6327c314827a20e0d7a1525398b38526fe28fd36e1",
		"input": "0x",
		"nonce": "0x12",
		"to": null,
		"transactionIndex": "0x12",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0x5bb64e885f196f7b515e62e3b90496864d960e2f5e0d7ad88550fa1c875ca691",
		"input": "0x",
		"nonce": "0x13",
		"to": "0x3632323232656162393565396564323963346266",
		"transactionIndex": "0x13",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0x6f4308b3c98db2db215d02c0df24472a215df7aa283261fcb06a6c9f796df9af",
		"input": "0x",
		"nonce": "0x14",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0x14",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	},
	{
		"blockHash": "0xc74d8c04d4d2f2e4ed9cd1731387248367cea7f149731b7a015371b220ffa0fb",
		"blockNumber": "0x4",
		"from": "0x3936663364636533666637396132333733653330",
		"gas": "0x2faf080",
		"gasPrice": "0x5d21dba00",
		"hash": "0x1df88d113f0c5833c1f7264687cd6ac43888c232600ffba8d3a7d89bb5013e71",
		"input": "0xf8ad80b8aaf8a8a0072409b14b96f9d7dbf4788dbc68c5d30bd5fac1431c299e0ab55c92e70a28a4a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a00000000000000000000000000000000000000000000000000000000000000000a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a00000000000000000000000000000000000000000000000000000000000000000808080",
		"nonce": "0x15",
		"to": "0x3936663364636533666637396132333733653330",
		"transactionIndex": "0x15",
		"value": "0x0",
		"type": "0x0",
		"v": "0x1",
		"r": "0x2",
		"s": "0x3"
	}
]`

// TestEthereumAPI_PendingTransactionstests PendingTransactions.
func testEthereumAPI_PendingTransactions(t *testing.T) {
	mockCtrl, mockBackend, api := testInitForEthApi(t)
	_, txs, txHashMap, _, _ := createTestData(t, nil)

	mockAccountManager := mock_accounts.NewMockAccountManager(mockCtrl)
	mockBackend.EXPECT().AccountManager().Return(mockAccountManager)

	mockBackend.EXPECT().GetPoolTransactions().Return(txs, nil)

	wallets := make([]accounts.Wallet, 1)
	wallets[0] = NewMockWallet(txs)
	mockAccountManager.EXPECT().Wallets().Return(wallets)

	pendingTxs, err := api.PendingTransactions()
	if err != nil {
		t.Fatal(err)
	}

	for _, pt := range pendingTxs {
		checkEthRPCTransactionFormat(t, nil, pt, txHashMap[pt.Hash], 0)
	}

	mockCtrl.Finish()
}

// TestEthereumAPI_GetTransactionReceipt tests GetTransactionReceipt.
func TestEthereumAPI_GetTransactionReceipt(t *testing.T) {
	mockCtrl, mockBackend, api := testInitForEthApi(t)
	block, txs, txHashMap, receiptMap, receipts := createTestData(t, nil)

	// Mock Backend functions.
	mockBackend.EXPECT().GetTxLookupInfoAndReceipt(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, *types.Receipt) {
			txLookupInfo := txHashMap[hash]
			idx := txLookupInfo.Nonce() // Assume idx of the transaction is nonce
			return txLookupInfo, block.Hash(), block.NumberU64(), idx, receiptMap[hash]
		},
	).Times(txs.Len())
	mockBackend.EXPECT().GetBlockReceipts(gomock.Any(), gomock.Any()).Return(receipts).Times(txs.Len())
	mockBackend.EXPECT().HeaderByHash(gomock.Any(), block.Hash()).Return(block.Header(), nil).Times(txs.Len())

	// Get receipt for each transaction types.
	for i := 0; i < txs.Len(); i++ {
		receipt, err := api.GetTransactionReceipt(context.Background(), txs[i].Hash())
		if err != nil {
			t.Fatal(err)
		}
		txIdx := uint64(i)
		checkEthTransactionReceiptFormat(t, block, receipts, receipt, RpcOutputReceipt(block.Header(), txs[i], block.Hash(), block.NumberU64(), txIdx, receiptMap[txs[i].Hash()]), txIdx)
	}

	mockCtrl.Finish()
}

func testInitForEthApi(t *testing.T) (*gomock.Controller, *mock_api.MockBackend, EthereumAPI) {
	mockCtrl := gomock.NewController(t)
	mockBackend := mock_api.NewMockBackend(mockCtrl)

	blockchain.InitDeriveSha(dummyChainConfigForEthereumAPITest)

	api := EthereumAPI{
		publicTransactionPoolAPI: NewPublicTransactionPoolAPI(mockBackend, new(AddrLocker)),
		publicKlayAPI:            NewPublicKlayAPI(mockBackend),
		publicBlockChainAPI:      NewPublicBlockChainAPI(mockBackend),
	}
	return mockCtrl, mockBackend, api
}

func checkEthRPCTransactionFormat(t *testing.T, block *types.Block, ethTx *EthRPCTransaction, tx *types.Transaction, expectedIndex hexutil.Uint64) {
	// All Klaytn transaction types must be returned as TxTypeLegacyTransaction types.
	assert.Equal(t, types.TxType(ethTx.Type), types.TxTypeLegacyTransaction)

	// Check the data of common fields of the transaction.
	from := getFrom(tx)
	assert.Equal(t, from, ethTx.From)
	assert.Equal(t, hexutil.Uint64(tx.Gas()), ethTx.Gas)
	assert.Equal(t, tx.GasPrice(), ethTx.GasPrice.ToInt())
	assert.Equal(t, tx.Hash(), ethTx.Hash)
	assert.Equal(t, tx.GetTxInternalData().RawSignatureValues()[0].V, ethTx.V.ToInt())
	assert.Equal(t, tx.GetTxInternalData().RawSignatureValues()[0].R, ethTx.R.ToInt())
	assert.Equal(t, tx.GetTxInternalData().RawSignatureValues()[0].S, ethTx.S.ToInt())
	assert.Equal(t, hexutil.Uint64(tx.Nonce()), ethTx.Nonce)

	// Check the optional field of Klaytn transactions.
	assert.Equal(t, 0, bytes.Compare(ethTx.Input, tx.Data()))

	to := tx.To()
	switch tx.Type() {
	case types.TxTypeAccountUpdate, types.TxTypeFeeDelegatedAccountUpdate, types.TxTypeFeeDelegatedAccountUpdateWithRatio,
		types.TxTypeCancel, types.TxTypeFeeDelegatedCancel, types.TxTypeFeeDelegatedCancelWithRatio,
		types.TxTypeChainDataAnchoring, types.TxTypeFeeDelegatedChainDataAnchoring, types.TxTypeFeeDelegatedChainDataAnchoringWithRatio:
		assert.Equal(t, &from, ethTx.To)
	default:
		assert.Equal(t, to, ethTx.To)
	}
	value := tx.Value()
	assert.Equal(t, value, ethTx.Value.ToInt())

	// If it is not a pending transaction and has already been processed and added into a block,
	// the following fields should be returned.
	if block != nil {
		assert.Equal(t, block.Hash().String(), ethTx.BlockHash.String())
		assert.Equal(t, block.NumberU64(), ethTx.BlockNumber.ToInt().Uint64())
		assert.Equal(t, expectedIndex, *ethTx.TransactionIndex)
	}

	// Fields additionally used for Ethereum transaction types are not used
	// when returning Klaytn transactions.
	assert.Equal(t, true, reflect.ValueOf(ethTx.Accesses).IsNil())
	assert.Equal(t, true, reflect.ValueOf(ethTx.ChainID).IsNil())
	assert.Equal(t, true, reflect.ValueOf(ethTx.GasFeeCap).IsNil())
	assert.Equal(t, true, reflect.ValueOf(ethTx.GasTipCap).IsNil())
}

func checkEthTransactionReceiptFormat(t *testing.T, block *types.Block, receipts []*types.Receipt, ethReceipt map[string]interface{}, kReceipt map[string]interface{}, idx uint64) {
	tx := block.Transactions()[idx]

	// Check the common receipt fields.
	blockHash, ok := ethReceipt["blockHash"]
	if !ok {
		t.Fatal("blockHash is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, blockHash, kReceipt["blockHash"])

	blockNumber, ok := ethReceipt["blockNumber"]
	if !ok {
		t.Fatal("blockNumber is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, blockNumber.(hexutil.Uint64), hexutil.Uint64(kReceipt["blockNumber"].(*hexutil.Big).ToInt().Uint64()))

	transactionHash, ok := ethReceipt["transactionHash"]
	if !ok {
		t.Fatal("transactionHash is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, transactionHash, kReceipt["transactionHash"])

	transactionIndex, ok := ethReceipt["transactionIndex"]
	if !ok {
		t.Fatal("transactionIndex is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, transactionIndex, hexutil.Uint64(kReceipt["transactionIndex"].(hexutil.Uint)))

	from, ok := ethReceipt["from"]
	if !ok {
		t.Fatal("from is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, from, kReceipt["from"])

	// Klaytn transactions that do not use the 'To' field
	// fill in 'To' with from during converting format.
	toInTx := tx.To()
	fromAddress := getFrom(tx)
	to, ok := ethReceipt["to"]
	if !ok {
		t.Fatal("to is not defined in Ethereum transaction receipt format.")
	}
	switch tx.Type() {
	case types.TxTypeAccountUpdate, types.TxTypeFeeDelegatedAccountUpdate, types.TxTypeFeeDelegatedAccountUpdateWithRatio,
		types.TxTypeCancel, types.TxTypeFeeDelegatedCancel, types.TxTypeFeeDelegatedCancelWithRatio,
		types.TxTypeChainDataAnchoring, types.TxTypeFeeDelegatedChainDataAnchoring, types.TxTypeFeeDelegatedChainDataAnchoringWithRatio:
		assert.Equal(t, &fromAddress, to)
	default:
		assert.Equal(t, toInTx, to)
	}

	gasUsed, ok := ethReceipt["gasUsed"]
	if !ok {
		t.Fatal("gasUsed is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, gasUsed, kReceipt["gasUsed"])

	// Compare with the calculated cumulative gas used value
	// to check whether the cumulativeGasUsed value is calculated properly.
	cumulativeGasUsed, ok := ethReceipt["cumulativeGasUsed"]
	if !ok {
		t.Fatal("cumulativeGasUsed is not defined in Ethereum transaction receipt format.")
	}
	calculatedCumulativeGas := uint64(0)
	for i := 0; i <= int(idx); i++ {
		calculatedCumulativeGas += receipts[i].GasUsed
	}
	assert.Equal(t, cumulativeGasUsed, hexutil.Uint64(calculatedCumulativeGas))

	contractAddress, ok := ethReceipt["contractAddress"]
	if !ok {
		t.Fatal("contractAddress is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, contractAddress, kReceipt["contractAddress"])

	logs, ok := ethReceipt["logs"]
	if !ok {
		t.Fatal("logs is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, logs, kReceipt["logs"])

	logsBloom, ok := ethReceipt["logsBloom"]
	if !ok {
		t.Fatal("logsBloom is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, logsBloom, kReceipt["logsBloom"])

	typeInt, ok := ethReceipt["type"]
	if !ok {
		t.Fatal("type is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, types.TxType(typeInt.(hexutil.Uint)), types.TxTypeLegacyTransaction)

	effectiveGasPrice, ok := ethReceipt["effectiveGasPrice"]
	if !ok {
		t.Fatal("effectiveGasPrice is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, effectiveGasPrice, hexutil.Uint64(kReceipt["gasPrice"].(*hexutil.Big).ToInt().Uint64()))

	status, ok := ethReceipt["status"]
	if !ok {
		t.Fatal("status is not defined in Ethereum transaction receipt format.")
	}
	assert.Equal(t, status, kReceipt["status"])

	// Check the receipt fields that should be removed.
	var shouldNotExisted []string
	shouldNotExisted = append(shouldNotExisted, "gas", "gasPrice", "senderTxHash", "signatures", "txError", "typeInt", "feePayer", "feePayerSignatures", "feeRatio", "input", "value", "codeFormat", "humanReadable", "key", "inputJSON")
	for i := 0; i < len(shouldNotExisted); i++ {
		k := shouldNotExisted[i]
		_, ok = ethReceipt[k]
		if ok {
			t.Fatal(k, " should not be defined in the Ethereum transaction receipt format.")
		}
	}
}

// TODO: Add TxTypeEthereumAccessList and TxTypeEthereumDynamicFee
func createTestData(t *testing.T, header *types.Header) (*types.Block, types.Transactions, map[common.Hash]*types.Transaction, map[common.Hash]*types.Receipt, []*types.Receipt) {
	var txs types.Transactions

	gasPrice := big.NewInt(25 * params.Ston)
	deployData := "0x60806040526000805534801561001457600080fd5b506101ea806100246000396000f30060806040526004361061006d576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806306661abd1461007257806342cbb15c1461009d578063767800de146100c8578063b22636271461011f578063d14e62b814610150575b600080fd5b34801561007e57600080fd5b5061008761017d565b6040518082815260200191505060405180910390f35b3480156100a957600080fd5b506100b2610183565b6040518082815260200191505060405180910390f35b3480156100d457600080fd5b506100dd61018b565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561012b57600080fd5b5061014e60048036038101908080356000191690602001909291905050506101b1565b005b34801561015c57600080fd5b5061017b600480360381019080803590602001909291905050506101b4565b005b60005481565b600043905090565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b50565b80600081905550505600a165627a7a7230582053c65686a3571c517e2cf4f741d842e5ee6aa665c96ce70f46f9a594794f11eb0029"
	executeData := "0xa9059cbb0000000000000000000000008a4c9c443bb0645df646a2d5bb55def0ed1e885a0000000000000000000000000000000000000000000000000000000000003039"
	var anchorData []byte

	txHashMap := make(map[common.Hash]*types.Transaction)
	receiptMap := make(map[common.Hash]*types.Receipt)
	var receipts []*types.Receipt

	// Create test data for chainDataAnchoring tx
	{
		dummyBlock := types.NewBlock(&types.Header{}, nil, nil)
		scData, err := types.NewAnchoringDataType0(dummyBlock, 0, uint64(dummyBlock.Transactions().Len()))
		if err != nil {
			t.Fatal(err)
		}
		anchorData, _ = rlp.EncodeToBytes(scData)
	}

	// Make test transactions data
	{
		// TxTypeLegacyTransaction
		values := map[types.TxValueKeyType]interface{}{
			// Simply set the nonce to txs.Len() to have a different nonce for each transaction type.
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyTo:       common.StringToAddress("0xe0680cfce04f80a386f1764a55c833b108770490"),
			types.TxValueKeyAmount:   big.NewInt(0),
			types.TxValueKeyGasLimit: uint64(30000000),
			types.TxValueKeyGasPrice: gasPrice,
			types.TxValueKeyData:     []byte("0xe3197e8f000000000000000000000000e0bef99b4a22286e27630b485d06c5a147cee931000000000000000000000000158beff8c8cdebd64654add5f6a1d9937e73536c0000000000000000000000000000000000000000000029bb5e7fb6beae32cf8000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000180000000000000000000000000000000000000000000000000001b60fb4614a22e000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000000000000000000000000000158beff8c8cdebd64654add5f6a1d9937e73536c00000000000000000000000074ba03198fed2b15a51af242b9c63faf3c8f4d3400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeLegacyTransaction, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])

	}
	{
		// TxTypeValueTransfer
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyFrom:     common.StringToAddress("0x520af902892196a3449b06ead301daeaf67e77e8"),
			types.TxValueKeyTo:       common.StringToAddress("0xa06fa690d92788cac4953da5f2dfbc4a2b3871db"),
			types.TxValueKeyAmount:   big.NewInt(5),
			types.TxValueKeyGasLimit: uint64(10000000),
			types.TxValueKeyGasPrice: gasPrice,
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeValueTransfer, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeValueTransferMemo
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyFrom:     common.StringToAddress("0xc05e11f9075d453b4fc87023f815fa6a63f7aa0c"),
			types.TxValueKeyTo:       common.StringToAddress("0xb5a2d79e9228f3d278cb36b5b15930f24fe8bae8"),
			types.TxValueKeyAmount:   big.NewInt(3),
			types.TxValueKeyGasLimit: uint64(20000000),
			types.TxValueKeyGasPrice: gasPrice,
			types.TxValueKeyData:     []byte(string("hello")),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeValueTransferMemo, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeAccountUpdate
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:      uint64(txs.Len()),
			types.TxValueKeyFrom:       common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit:   uint64(20000000),
			types.TxValueKeyGasPrice:   gasPrice,
			types.TxValueKeyAccountKey: accountkey.NewAccountKeyLegacy(),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeAccountUpdate, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeSmartContractDeploy
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         uint64(txs.Len()),
			types.TxValueKeyFrom:          common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyTo:            (*common.Address)(nil),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      uint64(100000000),
			types.TxValueKeyGasPrice:      gasPrice,
			types.TxValueKeyData:          common.Hex2Bytes(deployData),
			types.TxValueKeyHumanReadable: false,
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		r := createReceipt(t, tx, tx.Gas())
		fromAddress, err := tx.From()
		if err != nil {
			t.Fatal(err)
		}
		tx.FillContractAddress(fromAddress, r)
		receiptMap[tx.Hash()] = r
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeSmartContractExecution
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyFrom:     common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyTo:       common.StringToAddress("0x00ca1eee49a4d2b04e6562222eab95e9ed29c4bf"),
			types.TxValueKeyAmount:   big.NewInt(0),
			types.TxValueKeyGasLimit: uint64(50000000),
			types.TxValueKeyGasPrice: gasPrice,
			types.TxValueKeyData:     common.Hex2Bytes(executeData),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractExecution, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeCancel
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyFrom:     common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit: uint64(50000000),
			types.TxValueKeyGasPrice: gasPrice,
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeCancel, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeChainDataAnchoring
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:        uint64(txs.Len()),
			types.TxValueKeyFrom:         common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit:     uint64(50000000),
			types.TxValueKeyGasPrice:     gasPrice,
			types.TxValueKeyAnchoredData: anchorData,
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeChainDataAnchoring, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedValueTransfer
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyFrom:     common.StringToAddress("0x520af902892196a3449b06ead301daeaf67e77e8"),
			types.TxValueKeyTo:       common.StringToAddress("0xa06fa690d92788cac4953da5f2dfbc4a2b3871db"),
			types.TxValueKeyAmount:   big.NewInt(5),
			types.TxValueKeyGasLimit: uint64(10000000),
			types.TxValueKeyGasPrice: gasPrice,
			types.TxValueKeyFeePayer: common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransfer, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedValueTransferMemo
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyFrom:     common.StringToAddress("0xc05e11f9075d453b4fc87023f815fa6a63f7aa0c"),
			types.TxValueKeyTo:       common.StringToAddress("0xb5a2d79e9228f3d278cb36b5b15930f24fe8bae8"),
			types.TxValueKeyAmount:   big.NewInt(3),
			types.TxValueKeyGasLimit: uint64(20000000),
			types.TxValueKeyGasPrice: gasPrice,
			types.TxValueKeyData:     []byte(string("hello")),
			types.TxValueKeyFeePayer: common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransferMemo, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx

		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedAccountUpdate
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:      uint64(txs.Len()),
			types.TxValueKeyFrom:       common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit:   uint64(20000000),
			types.TxValueKeyGasPrice:   gasPrice,
			types.TxValueKeyAccountKey: accountkey.NewAccountKeyLegacy(),
			types.TxValueKeyFeePayer:   common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedAccountUpdate, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedSmartContractDeploy
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         uint64(txs.Len()),
			types.TxValueKeyFrom:          common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyTo:            (*common.Address)(nil),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      uint64(100000000),
			types.TxValueKeyGasPrice:      gasPrice,
			types.TxValueKeyData:          common.Hex2Bytes(deployData),
			types.TxValueKeyHumanReadable: false,
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
			types.TxValueKeyFeePayer:      common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		r := createReceipt(t, tx, tx.Gas())
		fromAddress, err := tx.From()
		if err != nil {
			t.Fatal(err)
		}
		tx.FillContractAddress(fromAddress, r)
		receiptMap[tx.Hash()] = r
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedSmartContractExecution
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyFrom:     common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyTo:       common.StringToAddress("0x00ca1eee49a4d2b04e6562222eab95e9ed29c4bf"),
			types.TxValueKeyAmount:   big.NewInt(0),
			types.TxValueKeyGasLimit: uint64(50000000),
			types.TxValueKeyGasPrice: gasPrice,
			types.TxValueKeyData:     common.Hex2Bytes(executeData),
			types.TxValueKeyFeePayer: common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedSmartContractExecution, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedCancel
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:    uint64(txs.Len()),
			types.TxValueKeyFrom:     common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit: uint64(50000000),
			types.TxValueKeyGasPrice: gasPrice,
			types.TxValueKeyFeePayer: common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedCancel, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedChainDataAnchoring
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:        uint64(txs.Len()),
			types.TxValueKeyFrom:         common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit:     uint64(50000000),
			types.TxValueKeyGasPrice:     gasPrice,
			types.TxValueKeyAnchoredData: anchorData,
			types.TxValueKeyFeePayer:     common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedChainDataAnchoring, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx

		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedValueTransferWithRatio
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:              uint64(txs.Len()),
			types.TxValueKeyFrom:               common.StringToAddress("0x520af902892196a3449b06ead301daeaf67e77e8"),
			types.TxValueKeyTo:                 common.StringToAddress("0xa06fa690d92788cac4953da5f2dfbc4a2b3871db"),
			types.TxValueKeyAmount:             big.NewInt(5),
			types.TxValueKeyGasLimit:           uint64(10000000),
			types.TxValueKeyGasPrice:           gasPrice,
			types.TxValueKeyFeePayer:           common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
			types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(20),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransferWithRatio, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedValueTransferMemoWithRatio
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:              uint64(txs.Len()),
			types.TxValueKeyFrom:               common.StringToAddress("0xc05e11f9075d453b4fc87023f815fa6a63f7aa0c"),
			types.TxValueKeyTo:                 common.StringToAddress("0xb5a2d79e9228f3d278cb36b5b15930f24fe8bae8"),
			types.TxValueKeyAmount:             big.NewInt(3),
			types.TxValueKeyGasLimit:           uint64(20000000),
			types.TxValueKeyGasPrice:           gasPrice,
			types.TxValueKeyData:               []byte(string("hello")),
			types.TxValueKeyFeePayer:           common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
			types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(20),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransferMemoWithRatio, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedAccountUpdateWithRatio
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:              uint64(txs.Len()),
			types.TxValueKeyFrom:               common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit:           uint64(20000000),
			types.TxValueKeyGasPrice:           gasPrice,
			types.TxValueKeyAccountKey:         accountkey.NewAccountKeyLegacy(),
			types.TxValueKeyFeePayer:           common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
			types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(20),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedAccountUpdateWithRatio, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedSmartContractDeployWithRatio
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:              uint64(txs.Len()),
			types.TxValueKeyFrom:               common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyTo:                 (*common.Address)(nil),
			types.TxValueKeyAmount:             big.NewInt(0),
			types.TxValueKeyGasLimit:           uint64(100000000),
			types.TxValueKeyGasPrice:           gasPrice,
			types.TxValueKeyData:               common.Hex2Bytes(deployData),
			types.TxValueKeyHumanReadable:      false,
			types.TxValueKeyCodeFormat:         params.CodeFormatEVM,
			types.TxValueKeyFeePayer:           common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
			types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(20),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedSmartContractDeployWithRatio, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		r := createReceipt(t, tx, tx.Gas())
		fromAddress, err := tx.From()
		if err != nil {
			t.Fatal(err)
		}
		tx.FillContractAddress(fromAddress, r)
		receiptMap[tx.Hash()] = r
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedSmartContractExecutionWithRatio
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:              uint64(txs.Len()),
			types.TxValueKeyFrom:               common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyTo:                 common.StringToAddress("0x00ca1eee49a4d2b04e6562222eab95e9ed29c4bf"),
			types.TxValueKeyAmount:             big.NewInt(0),
			types.TxValueKeyGasLimit:           uint64(50000000),
			types.TxValueKeyGasPrice:           gasPrice,
			types.TxValueKeyData:               common.Hex2Bytes(executeData),
			types.TxValueKeyFeePayer:           common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
			types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(20),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedSmartContractExecutionWithRatio, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedCancelWithRatio
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:              uint64(txs.Len()),
			types.TxValueKeyFrom:               common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit:           uint64(50000000),
			types.TxValueKeyGasPrice:           gasPrice,
			types.TxValueKeyFeePayer:           common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
			types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(20),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedCancelWithRatio, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeFeeDelegatedChainDataAnchoringWithRatio
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:              uint64(txs.Len()),
			types.TxValueKeyFrom:               common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			types.TxValueKeyGasLimit:           uint64(50000000),
			types.TxValueKeyGasPrice:           gasPrice,
			types.TxValueKeyAnchoredData:       anchorData,
			types.TxValueKeyFeePayer:           common.StringToAddress("0xa142f7b24a618778165c9b06e15a61f100c51400"),
			types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(20),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedChainDataAnchoringWithRatio, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		feePayerSignatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(3), R: big.NewInt(4), S: big.NewInt(5)},
			&types.TxSignature{V: big.NewInt(4), R: big.NewInt(5), S: big.NewInt(6)},
		}
		tx.SetFeePayerSignatures(feePayerSignatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}

	// Create a block which includes all transaction data.
	var block *types.Block
	if header != nil {
		block = types.NewBlock(header, txs, receipts)
	} else {
		block = types.NewBlock(&types.Header{Number: big.NewInt(1)}, txs, nil)
	}

	return block, txs, txHashMap, receiptMap, receipts
}

func createEthereumTypedTestData(t *testing.T, header *types.Header) (*types.Block, types.Transactions, map[common.Hash]*types.Transaction, map[common.Hash]*types.Receipt, []*types.Receipt) {
	var txs types.Transactions

	gasPrice := big.NewInt(25 * params.Ston)
	deployData := "0x60806040526000805534801561001457600080fd5b506101ea806100246000396000f30060806040526004361061006d576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806306661abd1461007257806342cbb15c1461009d578063767800de146100c8578063b22636271461011f578063d14e62b814610150575b600080fd5b34801561007e57600080fd5b5061008761017d565b6040518082815260200191505060405180910390f35b3480156100a957600080fd5b506100b2610183565b6040518082815260200191505060405180910390f35b3480156100d457600080fd5b506100dd61018b565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561012b57600080fd5b5061014e60048036038101908080356000191690602001909291905050506101b1565b005b34801561015c57600080fd5b5061017b600480360381019080803590602001909291905050506101b4565b005b60005481565b600043905090565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b50565b80600081905550505600a165627a7a7230582053c65686a3571c517e2cf4f741d842e5ee6aa665c96ce70f46f9a594794f11eb0029"
	accessList := types.AccessList{
		types.AccessTuple{
			Address: common.StringToAddress("0x23a519a88e79fbc0bab796f3dce3ff79a2373e30"),
			StorageKeys: []common.Hash{
				common.HexToHash("0xa145cd642157a5df01f5bc3837a1bb59b3dcefbbfad5ec435919780aebeaba2b"),
				common.HexToHash("0x12e2c26dca2fb2b8879f54a5ea1604924edf0e37965c2be8aa6133b75818da40"),
			},
		},
	}
	chainId := new(big.Int).SetUint64(2019)

	txHashMap := make(map[common.Hash]*types.Transaction)
	receiptMap := make(map[common.Hash]*types.Receipt)
	var receipts []*types.Receipt

	// Make test transactions data
	{
		// TxTypeEthereumAccessList
		to := common.StringToAddress("0xb5a2d79e9228f3d278cb36b5b15930f24fe8bae8")
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:      uint64(txs.Len()),
			types.TxValueKeyTo:         &to,
			types.TxValueKeyAmount:     big.NewInt(10),
			types.TxValueKeyGasLimit:   uint64(50000000),
			types.TxValueKeyData:       common.Hex2Bytes(deployData),
			types.TxValueKeyGasPrice:   gasPrice,
			types.TxValueKeyAccessList: accessList,
			types.TxValueKeyChainID:    chainId,
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeEthereumAccessList, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(1), R: big.NewInt(2), S: big.NewInt(3)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}
	{
		// TxTypeEthereumDynamicFee
		to := common.StringToAddress("0xb5a2d79e9228f3d278cb36b5b15930f24fe8bae8")
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:      uint64(txs.Len()),
			types.TxValueKeyTo:         &to,
			types.TxValueKeyAmount:     big.NewInt(3),
			types.TxValueKeyGasLimit:   uint64(50000000),
			types.TxValueKeyData:       common.Hex2Bytes(deployData),
			types.TxValueKeyGasTipCap:  gasPrice,
			types.TxValueKeyGasFeeCap:  gasPrice,
			types.TxValueKeyAccessList: accessList,
			types.TxValueKeyChainID:    chainId,
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeEthereumDynamicFee, values)
		assert.Equal(t, nil, err)

		signatures := types.TxSignatures{
			&types.TxSignature{V: big.NewInt(2), R: big.NewInt(3), S: big.NewInt(4)},
		}
		tx.SetSignature(signatures)

		txs = append(txs, tx)
		txHashMap[tx.Hash()] = tx
		// For testing, set GasUsed with tx.Gas()
		receiptMap[tx.Hash()] = createReceipt(t, tx, tx.Gas())
		receipts = append(receipts, receiptMap[tx.Hash()])
	}

	// Create a block which includes all transaction data.
	var block *types.Block
	if header != nil {
		block = types.NewBlock(header, txs, receipts)
	} else {
		block = types.NewBlock(&types.Header{Number: big.NewInt(1)}, txs, nil)
	}

	return block, txs, txHashMap, receiptMap, receipts
}

func createReceipt(t *testing.T, tx *types.Transaction, gasUsed uint64) *types.Receipt {
	rct := types.NewReceipt(uint(0), tx.Hash(), gasUsed)
	rct.Logs = []*types.Log{}
	rct.Bloom = types.Bloom{}
	return rct
}

// MockDatabaseManager is a mock of database.DBManager interface for overriding the ReadTxAndLookupInfo function.
type MockDatabaseManager struct {
	database.DBManager

	txHashMap     map[common.Hash]*types.Transaction
	blockData     *types.Block
	queryFromPool bool
}

// GetTxLookupInfoAndReceipt retrieves a tx and lookup info and receipt for a given transaction hash.
func (dbm *MockDatabaseManager) ReadTxAndLookupInfo(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	// If queryFromPool, return nil to query from pool after this function
	if dbm.queryFromPool {
		return nil, common.Hash{}, 0, 0
	}

	txFromHashMap := dbm.txHashMap[hash]
	if txFromHashMap == nil {
		return nil, common.Hash{}, 0, 0
	}
	return txFromHashMap, dbm.blockData.Hash(), dbm.blockData.NumberU64(), txFromHashMap.Nonce()
}

// MockWallet is a mock of accounts.Wallet interface for overriding the Accounts function.
type MockWallet struct {
	accounts.Wallet

	accounts []accounts.Account
}

// NewMockWallet prepares accounts based on tx from.
func NewMockWallet(txs types.Transactions) *MockWallet {
	mw := &MockWallet{}

	for _, t := range txs {
		mw.accounts = append(mw.accounts, accounts.Account{Address: getFrom(t)})
	}
	return mw
}

// Accounts implements accounts.Wallet, returning an account list.
func (mw *MockWallet) Accounts() []accounts.Account {
	return mw.accounts
}

// TestEthTransactionArgs_setDefaults tests setDefaults method of EthTransactionArgs.
func TestEthTransactionArgs_setDefaults(t *testing.T) {
	_, mockBackend, _ := testInitForEthApi(t)
	// To clarify the exact scope of this test, it is assumed that the user must fill in the gas.
	// Because when user does not specify gas, it calls estimateGas internally and it requires
	// many backend calls which are not directly related with this test.
	gas := hexutil.Uint64(1000000)
	from := common.HexToAddress("0x2eaad2bf70a070aaa2e007beee99c6148f47718e")
	poolNonce := uint64(1)
	accountNonce := uint64(5)
	to := common.HexToAddress("0x9712f943b296758aaae79944ec975884188d3a96")
	byteCode := common.Hex2Bytes("6080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680632e64cec114604e5780636057361d146076575b600080fd5b348015605957600080fd5b50606060a0565b6040518082815260200191505060405180910390f35b348015608157600080fd5b50609e6004803603810190808035906020019092919050505060a9565b005b60008054905090565b80600081905550505600a165627a7a723058207783dba41884f73679e167576362b7277f88458815141651f48ca38c25b498f80029")
	unitPrice := new(big.Int).SetUint64(dummyChainConfigForEthereumAPITest.UnitPrice)
	value := new(big.Int).SetUint64(500)
	testSet := []struct {
		txArgs              EthTransactionArgs
		expectedResult      EthTransactionArgs
		dynamicFeeParamsSet bool
		nonceSet            bool
		chainIdSet          bool
		expectedError       error
	}{
		{
			txArgs: EthTransactionArgs{
				From:                 nil,
				To:                   nil,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         nil,
				MaxPriorityFeePerGas: nil,
				Value:                nil,
				Nonce:                nil,
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult: EthTransactionArgs{
				From:                 nil,
				To:                   nil,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(new(big.Int)),
				Nonce:                (*hexutil.Uint64)(&poolNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(dummyChainConfigForEthereumAPITest.ChainID),
			},
			dynamicFeeParamsSet: false,
			nonceSet:            false,
			chainIdSet:          false,
			expectedError:       nil,
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             (*hexutil.Big)(unitPrice),
				MaxFeePerGas:         nil,
				MaxPriorityFeePerGas: nil,
				Value:                (*hexutil.Big)(value),
				Nonce:                nil,
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             (*hexutil.Big)(unitPrice),
				MaxFeePerGas:         nil,
				MaxPriorityFeePerGas: nil,
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&poolNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(dummyChainConfigForEthereumAPITest.ChainID),
			},
			dynamicFeeParamsSet: false,
			nonceSet:            false,
			chainIdSet:          false,
			expectedError:       nil,
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(new(big.Int).SetUint64(1)),
				MaxPriorityFeePerGas: nil,
				Value:                (*hexutil.Big)(value),
				Nonce:                nil,
				Data:                 nil,
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult:      EthTransactionArgs{},
			dynamicFeeParamsSet: false,
			nonceSet:            false,
			chainIdSet:          false,
			expectedError:       fmt.Errorf("only %s is allowed to be used as maxFeePerGas and maxPriorityPerGas", unitPrice.Text(16)),
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         nil,
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                nil,
				Data:                 nil,
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&poolNonce),
				Data:                 nil,
				Input:                nil,
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(dummyChainConfigForEthereumAPITest.ChainID),
			},
			dynamicFeeParamsSet: false,
			nonceSet:            false,
			chainIdSet:          false,
			expectedError:       nil,
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         nil,
				MaxPriorityFeePerGas: (*hexutil.Big)(new(big.Int).SetUint64(1)),
				Value:                (*hexutil.Big)(value),
				Nonce:                nil,
				Data:                 nil,
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult:      EthTransactionArgs{},
			dynamicFeeParamsSet: false,
			nonceSet:            false,
			chainIdSet:          false,
			expectedError:       fmt.Errorf("only %s is allowed to be used as maxFeePerGas and maxPriorityPerGas", unitPrice.Text(16)),
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             (*hexutil.Big)(unitPrice),
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                nil,
				Data:                 nil,
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult:      EthTransactionArgs{},
			dynamicFeeParamsSet: false,
			nonceSet:            false,
			chainIdSet:          false,
			expectedError:       errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified"),
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                nil,
				Data:                 nil,
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&poolNonce),
				Data:                 nil,
				Input:                nil,
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(dummyChainConfigForEthereumAPITest.ChainID),
			},
			dynamicFeeParamsSet: true,
			nonceSet:            false,
			chainIdSet:          false,
			expectedError:       nil,
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         nil,
				MaxPriorityFeePerGas: nil,
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&accountNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&accountNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(dummyChainConfigForEthereumAPITest.ChainID),
			},
			dynamicFeeParamsSet: false,
			nonceSet:            true,
			chainIdSet:          false,
			expectedError:       nil,
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&accountNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              nil,
			},
			expectedResult: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&accountNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(dummyChainConfigForEthereumAPITest.ChainID),
			},
			dynamicFeeParamsSet: true,
			nonceSet:            true,
			chainIdSet:          false,
			expectedError:       nil,
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&accountNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(new(big.Int).SetUint64(1234)),
			},
			expectedResult: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&accountNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                nil,
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(new(big.Int).SetUint64(1234)),
			},
			dynamicFeeParamsSet: true,
			nonceSet:            true,
			chainIdSet:          true,
			expectedError:       nil,
		},
		{
			txArgs: EthTransactionArgs{
				From:                 &from,
				To:                   &to,
				Gas:                  &gas,
				GasPrice:             nil,
				MaxFeePerGas:         (*hexutil.Big)(unitPrice),
				MaxPriorityFeePerGas: (*hexutil.Big)(unitPrice),
				Value:                (*hexutil.Big)(value),
				Nonce:                (*hexutil.Uint64)(&accountNonce),
				Data:                 (*hexutil.Bytes)(&byteCode),
				Input:                (*hexutil.Bytes)(&[]byte{0x1}),
				AccessList:           nil,
				ChainID:              (*hexutil.Big)(new(big.Int).SetUint64(1234)),
			},
			expectedResult:      EthTransactionArgs{},
			dynamicFeeParamsSet: true,
			nonceSet:            true,
			chainIdSet:          true,
			expectedError:       errors.New(`both "data" and "input" are set and not equal. Please use "input" to pass transaction call data`),
		},
	}
	for _, test := range testSet {
		mockBackend.EXPECT().CurrentBlock().Return(
			types.NewBlockWithHeader(&types.Header{Number: new(big.Int).SetUint64(0)}),
		)
		mockBackend.EXPECT().SuggestPrice(gomock.Any()).Return(unitPrice, nil)
		if !test.dynamicFeeParamsSet {
			mockBackend.EXPECT().ChainConfig().Return(dummyChainConfigForEthereumAPITest)
		}
		if !test.nonceSet {
			mockBackend.EXPECT().GetPoolNonce(context.Background(), gomock.Any()).Return(poolNonce)
		}
		if !test.chainIdSet {
			mockBackend.EXPECT().ChainConfig().Return(dummyChainConfigForEthereumAPITest)
		}
		mockBackend.EXPECT().RPCGasCap().Return(nil)
		txArgs := test.txArgs
		err := txArgs.setDefaults(context.Background(), mockBackend)
		require.Equal(t, test.expectedError, err)
		if err == nil {
			require.Equal(t, test.expectedResult, txArgs)
		}
	}
}

func TestEthereumAPI_GetRawTransactionByHash(t *testing.T) {
	mockCtrl, mockBackend, api := testInitForEthApi(t)
	block, txs, txHashMap, _, _ := createEthereumTypedTestData(t, nil)

	// Define queryFromPool for ReadTxAndLookupInfo function return tx from hash map.
	// MockDatabaseManager will initiate data with txHashMap, block and queryFromPool.
	// If queryFromPool is true, MockDatabaseManager will return nil to query transactions from transaction pool,
	// otherwise return a transaction from txHashMap.
	mockDBManager := &MockDatabaseManager{txHashMap: txHashMap, blockData: block, queryFromPool: false}

	// Mock Backend functions.
	mockBackend.EXPECT().ChainDB().Return(mockDBManager).Times(txs.Len())

	for i := 0; i < txs.Len(); i++ {
		rawTx, err := api.GetRawTransactionByHash(context.Background(), txs[i].Hash())
		if err != nil {
			t.Fatal(err)
		}
		prefix := types.TxType(rawTx[0])
		// When get raw transaction by eth namespace API, EthereumTxTypeEnvelope must not be included.
		require.NotEqual(t, types.EthereumTxTypeEnvelope, prefix)
	}

	mockCtrl.Finish()
}

func TestEthereumAPI_GetRawTransactionByBlockNumberAndIndex(t *testing.T) {
	mockCtrl, mockBackend, api := testInitForEthApi(t)
	block, txs, _, _, _ := createEthereumTypedTestData(t, nil)

	// Mock Backend functions.
	mockBackend.EXPECT().BlockByNumber(gomock.Any(), gomock.Any()).Return(block, nil).Times(txs.Len())

	for i := 0; i < txs.Len(); i++ {
		rawTx := api.GetRawTransactionByBlockNumberAndIndex(context.Background(), rpc.BlockNumber(block.NumberU64()), hexutil.Uint(i))
		prefix := types.TxType(rawTx[0])
		// When get raw transaction by eth namespace API, EthereumTxTypeEnvelope must not be included.
		require.NotEqual(t, types.EthereumTxTypeEnvelope, prefix)
	}

	mockCtrl.Finish()
}

type testChainContext struct {
	header *types.Header
}

func (mc *testChainContext) Engine() consensus.Engine {
	return gxhash.NewFaker()
}

func (mc *testChainContext) GetHeader(common.Hash, uint64) *types.Header {
	return mc.header
}

// Contract C { constructor() { revert("hello"); } }
var codeRevertHello = "0x6080604052348015600f57600080fd5b5060405162461bcd60e51b815260206004820152600560248201526468656c6c6f60d81b604482015260640160405180910390fdfe"

func testEstimateGas(t *testing.T, mockBackend *mock_api.MockBackend, fnEstimateGas func(EthTransactionArgs) (hexutil.Uint64, error)) {
	chainConfig := &params.ChainConfig{}
	chainConfig.IstanbulCompatibleBlock = common.Big0
	chainConfig.LondonCompatibleBlock = common.Big0
	chainConfig.EthTxTypeCompatibleBlock = common.Big0
	chainConfig.MagmaCompatibleBlock = common.Big0
	var (
		// genesis
		account1 = common.HexToAddress("0xaaaa")
		account2 = common.HexToAddress("0xbbbb")
		account3 = common.HexToAddress("0xcccc")
		gspec    = &blockchain.Genesis{Alloc: blockchain.GenesisAlloc{
			account1: {Balance: big.NewInt(params.KLAY * 2)},
			account2: {Balance: common.Big0},
			account3: {Balance: common.Big0, Code: hexutil.MustDecode(codeRevertHello)},
		}, Config: chainConfig}

		// blockchain
		dbm    = database.NewMemoryDBManager()
		db     = state.NewDatabase(dbm)
		block  = gspec.MustCommit(dbm)
		header = block.Header()
		chain  = &testChainContext{header: header}

		// tx arguments
		KLAY     = hexutil.Big(*big.NewInt(params.KLAY))
		mKLAY    = hexutil.Big(*big.NewInt(params.KLAY / 1000))
		KLAY2_1  = hexutil.Big(*big.NewInt(params.KLAY*2 + 1))
		gas1000  = hexutil.Uint64(1000)
		gas40000 = hexutil.Uint64(40000)
		baddata  = hexutil.Bytes(hexutil.MustDecode("0xdeadbeef"))
	)

	any := gomock.Any()
	getStateAndHeader := func(...interface{}) (*state.StateDB, *types.Header, error) {
		// Return a new state for each call because the state is modified by EstimateGas.
		state, err := state.New(block.Root(), db, nil, nil)
		return state, header, err
	}
	getEVM := func(_ context.Context, msg blockchain.Message, state *state.StateDB, header *types.Header, vmConfig vm.Config) (*vm.EVM, func() error, error) {
		// Taken from node/cn/api_backend.go
		vmError := func() error { return nil }
		txContext := blockchain.NewEVMTxContext(msg, header)
		blockContext := blockchain.NewEVMBlockContext(header, chain, nil)
		return vm.NewEVM(blockContext, txContext, state, chainConfig, &vmConfig), vmError, nil
	}
	mockBackend.EXPECT().ChainConfig().Return(chainConfig).AnyTimes()
	mockBackend.EXPECT().RPCGasCap().Return(common.Big0).AnyTimes()
	mockBackend.EXPECT().StateAndHeaderByNumber(any, any).DoAndReturn(getStateAndHeader).AnyTimes()
	mockBackend.EXPECT().StateAndHeaderByNumberOrHash(any, any).DoAndReturn(getStateAndHeader).AnyTimes()
	mockBackend.EXPECT().GetEVM(any, any, any, any, any).DoAndReturn(getEVM).AnyTimes()

	testcases := []struct {
		args      EthTransactionArgs
		expectErr string
		expectGas uint64
	}{
		{ // simple transfer
			args: EthTransactionArgs{
				From:  &account1,
				To:    &account2,
				Value: &KLAY,
			},
			expectGas: 21000,
		},
		{ // simple transfer with insufficient funds with zero gasPrice
			args: EthTransactionArgs{
				From:  &account2, // sender has 0 KLAY
				To:    &account1,
				Value: &KLAY, // transfer 1 KLAY
			},
			expectErr: "insufficient balance for transfer",
		},
		{ // simple transfer with slightly insufficient funds with zero gasPrice
			// this testcase is to check whether the gas prefunded in EthDoCall is not too much
			args: EthTransactionArgs{
				From:  &account1, // sender has 2 KLAY
				To:    &account2,
				Value: &KLAY2_1, // transfer 2.0000...1 KLAY
			},
			expectErr: "insufficient balance for transfer",
		},
		{ // simple transfer with insufficient funds with nonzero gasPrice
			args: EthTransactionArgs{
				From:     &account2, // sender has 0 KLAY
				To:       &account1,
				Value:    &KLAY, // transfer 1 KLAY
				GasPrice: &mKLAY,
			},
			expectErr: "insufficient funds for transfer",
		},
		{ // simple transfer too high gasPrice
			args: EthTransactionArgs{
				From:     &account1, // sender has 2 KLAY
				To:       &account2,
				Value:    &KLAY,  // transfer 1 KLAY
				GasPrice: &mKLAY, // allowance = (2 - 1) / 0.001 = 1000 gas
			},
			expectErr: "gas required exceeds allowance",
		},
		{ // empty create
			args:      EthTransactionArgs{},
			expectGas: 53000,
		},
		{ // ignore too small gasLimit
			args: EthTransactionArgs{
				Gas: &gas1000,
			},
			expectGas: 53000,
		},
		{ // capped by gasLimit
			args: EthTransactionArgs{
				Gas: &gas40000,
			},
			expectErr: "gas required exceeds allowance",
		},
		{ // fails with VM error
			args: EthTransactionArgs{
				From: &account1,
				Data: &baddata,
			},
			expectErr: "VM error occurs while running smart contract",
		},
		{ // fails with contract revert
			args: EthTransactionArgs{
				From: &account1,
				To:   &account3,
			},
			expectErr: "execution reverted: hello",
		},
	}

	for i, tc := range testcases {
		gas, err := fnEstimateGas(tc.args)
		t.Logf("tc[%02d] = %d %v", i, gas, err)
		if len(tc.expectErr) > 0 {
			require.NotNil(t, err)
			assert.Contains(t, err.Error(), tc.expectErr, i)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, tc.expectGas, uint64(gas), i)
		}
	}
}

func TestEthereumAPI_EstimateGas(t *testing.T) {
	mockCtrl, mockBackend, api := testInitForEthApi(t)
	defer mockCtrl.Finish()

	testEstimateGas(t, mockBackend, func(args EthTransactionArgs) (hexutil.Uint64, error) {
		return api.EstimateGas(context.Background(), args, nil)
	})
}
