package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	mock_api "github.com/klaytn/klaytn/api/mocks"
	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/blockchain/types/accountkey"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/common/hexutil"
	mock_consensus "github.com/klaytn/klaytn/consensus/mocks"
	"github.com/klaytn/klaytn/networks/rpc"
	"github.com/klaytn/klaytn/params"
	"github.com/klaytn/klaytn/rlp"
	"github.com/stretchr/testify/assert"
)

// Generic values of various types
// Unrelated to blockchain data
var (
	bg = context.Background()

	hash0   = common.Hash{}
	hashAny = common.HexToHash("0x1234")
	num0    = rpc.BlockNumber(0)
	numAny  = rpc.BlockNumber(99)
	u64_0   = hexutil.Uint64(0)
	u64_Any = hexutil.Uint64(42)
	uint0   = hexutil.Uint(0)
	uintNil = (*hexutil.Uint)(nil)
	map0    = make(map[string]interface{})  // map with no elements
	mapNil  = (map[string]interface{})(nil) // nil ptr
)

// Sample blockchain data
var (
	// Chain config
	unitPrice = uint64(25_000_000_000) // 25 ston
	chainId   = big.NewInt(111111)

	// Node data
	nodeAddr = common.HexToAddress("0x9712f943b296758aaae79944ec975884188d3a96")

	// Block data
	td     = big.NewInt(5)
	number = big.NewInt(4)
	author = common.HexToAddress("0x2eaad2bf70a070aaa2e007beee99c6148f47718e")

	// Expected errors
	testErrUnknownHeader = fmt.Errorf("the block does not exist (block number: %d)", number.Uint64())
	testErrUnknownBlock  = fmt.Errorf("the header does not exist (block number: %d)", number.Uint64())

	// Dynamically generated depending on fork Rules
	chainConfig     *params.ChainConfig
	header          *types.Header
	ethHeaderMap    map[string]interface{}
	block           *types.Block
	ethBlockMapHash map[string]interface{}
	ethBlockMapFull map[string]interface{}

	londonConfig = &params.ChainConfig{
		ChainID:                 chainId,
		IstanbulCompatibleBlock: common.Big0,
		LondonCompatibleBlock:   common.Big0,
		UnitPrice:               unitPrice,
	}
)

// Inject data to API backend

type mockFunc func(ctrl *gomock.Controller, backend *mock_api.MockBackend)

var Any = gomock.Any()

// Stuff backend.BlockByNumber() and backend.BlockByHash() with valid block
func mockBlock(ctrl *gomock.Controller, backend *mock_api.MockBackend) {
	mockEngine := mock_consensus.NewMockEngine(ctrl)
	mockEngine.EXPECT().Author(Any).Return(author, nil).AnyTimes()

	backend.EXPECT().Engine().Return(mockEngine).AnyTimes()
	backend.EXPECT().GetTd(Any).Return(td).AnyTimes()
	backend.EXPECT().ChainConfig().Return(chainConfig).AnyTimes()

	backend.EXPECT().HeaderByNumber(Any, Any).Return(header, nil).AnyTimes()
	backend.EXPECT().HeaderByHash(Any, Any).Return(header, nil).AnyTimes()

	backend.EXPECT().BlockByNumber(Any, Any).Return(block, nil).AnyTimes()
	backend.EXPECT().BlockByHash(Any, Any).Return(block, nil).AnyTimes()
}

// Stuff backend.BlockByNumber() and backend.BlockByHash() with unknown block error
func mockNoBlock(ctrl *gomock.Controller, backend *mock_api.MockBackend) {
	backend.EXPECT().HeaderByNumber(Any, Any).Return(nil, testErrUnknownHeader).AnyTimes()
	backend.EXPECT().HeaderByHash(Any, Any).Return(nil, testErrUnknownHeader).AnyTimes()

	backend.EXPECT().BlockByNumber(Any, Any).Return(nil, testErrUnknownBlock).AnyTimes()
	backend.EXPECT().BlockByHash(Any, Any).Return(nil, testErrUnknownBlock).AnyTimes()
}

// Create test data

func refreshData(t *testing.T, config *params.ChainConfig) {
	blockchain.InitDeriveSha(config)
	chainConfig = config
	rules := chainConfig.Rules(big.NewInt(4))
	header, ethHeaderMap = createHeader(rules)
	block, ethBlockMapHash, ethBlockMapFull = createTestBlock(t, rules, header, ethHeaderMap)
}

func createHeader(rules params.Rules) (*types.Header, map[string]interface{}) {
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
		headerMap["mixHash"] = "0xdf117d1245dceaae0a47f05371b23cd0d0db963ff9d5c8ba768dc989f4c31883" // TODO: fix
		headerMap["size"] = "0x315"
	}
	return header, headerMap
}

func createTestBlock(t *testing.T, rules params.Rules, header *types.Header, headerMap map[string]interface{},
) (*types.Block, map[string]interface{}, map[string]interface{}) {
	block, _, _, _, _ := createTestData(t, header)
	blockMap := cloneMap(headerMap)

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
	assert.Nil(t, json.Unmarshal([]byte(txHashesJson), &txhashList))
	blockMapHash := cloneMap(blockMap)
	blockMapHash["transactions"] = txhashList

	var txObjectList []interface{}
	assert.Nil(t, json.Unmarshal([]byte(txObjectsJson), &txObjectList))
	for i := range txObjectList {
		txObjectList[i].(map[string]interface{})["blockHash"] = blockMap["hash"]
	}
	blockMapFull := cloneMap(blockMap)
	blockMapFull["transactions"] = txObjectList

	return block, blockMapHash, blockMapFull
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{})
	for k, v := range src {
		dst[k] = v
	}
	return dst
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

func createReceipt(t *testing.T, tx *types.Transaction, gasUsed uint64) *types.Receipt {
	rct := types.NewReceipt(uint(0), tx.Hash(), gasUsed)
	rct.Logs = []*types.Log{}
	rct.Bloom = types.Bloom{}
	return rct
}

var txHashesJson = `[
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
var txObjectsJson = `[
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
