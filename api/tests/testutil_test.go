package tests

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/klaytn/klaytn/api"
	mock_api "github.com/klaytn/klaytn/api/mocks"
	"github.com/klaytn/klaytn/governance"
	"github.com/klaytn/klaytn/storage/database"
	"github.com/stretchr/testify/assert"
)

// Test assert helpers
type testContext interface {
	T() *testing.T
	Method(name string) reflect.Value
}

func AssertErr(ctx testContext, expectedErr string, method string, params ...interface{}) {
	ctx.T().Run(method, func(t *testing.T) {
		_, err := callMethod(t, ctx.Method(method), params...)
		assert.ErrorContains(t, err, expectedErr)
	})
}

func AssertEq(ctx testContext, expected interface{}, method string, params ...interface{}) {
	ctx.T().Run(method, func(t *testing.T) {
		result, err := callMethod(t, ctx.Method(method), params...)
		assert.Nil(t, err)
		assert.Equal(t, expected, result)
	})
}

func AssertJson(ctx testContext, expected map[string]interface{}, method string, params ...interface{}) {
	ctx.T().Run(method, func(t *testing.T) {
		result, err := callMethod(t, ctx.Method(method), params...)
		assert.Nil(t, err)
		assert.Equal(t, expected, stringifyResult(t, result))
	})
}

func stringifyResult(t *testing.T, result interface{}) map[string]interface{} {
	resultJsonBytes, err := json.Marshal(result)
	assert.Nil(t, err)

	var resultMap map[string]interface{}
	assert.Nil(t, json.Unmarshal(resultJsonBytes, &resultMap))

	return resultMap
}

// EthereumAPI test context
type ethContext struct {
	t *testing.T

	mockCtrl    *gomock.Controller
	mockBackend *mock_api.MockBackend
	api         *api.EthereumAPI
}

func (ctx *ethContext) T() *testing.T { return ctx.t }

func (ctx *ethContext) Run(name string, f func(t *testing.T)) {
	ctx.t.Run(name, f)
}

func (ctx *ethContext) Method(name string) reflect.Value {
	return reflect.ValueOf(ctx.api).MethodByName(name)
}

func WithEth(t *testing.T, injects []mockFunc, callback func(ctx *ethContext)) {
	ctx := &ethContext{t: t}

	// Setup mock backend and API
	ctx.mockCtrl = gomock.NewController(t)
	defer ctx.mockCtrl.Finish()

	backend := mock_api.NewMockBackend(ctx.mockCtrl)
	ctx.mockBackend = backend

	for _, inject := range injects {
		inject(ctx.mockCtrl, ctx.mockBackend)
	}

	ctx.api = api.NewEthereumAPI()
	ctx.api.SetPublicBlockChainAPI(api.NewPublicBlockChainAPI(backend))
	ctx.api.SetPublicKlayAPI(api.NewPublicKlayAPI(backend))
	ctx.api.SetPublicTransactionPoolAPI(api.NewPublicTransactionPoolAPI(backend, new(api.AddrLocker)))
	ctx.api.SetGovernanceAPI(newTestGovernanceAPI())

	callback(ctx)
}

func newTestGovernanceAPI() *governance.GovernanceAPI {
	var (
		dbm    = database.NewMemoryDBManager()
		gov    = governance.NewMixedEngineNoInit(londonConfig, dbm)
		govAPI = governance.NewGovernanceAPI(gov)
	)
	gov.SetNodeAddress(nodeAddr)

	return govAPI
}

func callMethod(t *testing.T, methodPtr reflect.Value, params ...interface{}) (interface{}, error) {
	methodTy := methodPtr.Type()

	numIn := methodTy.NumIn()
	if numIn != len(params) {
		t.Fatalf("Unexpected parameters count: want=%d have=%d", numIn, len(params))
	}

	args := make([]reflect.Value, numIn)
	for i, param := range params {
		arg := reflect.ValueOf(param)
		if !arg.Type().AssignableTo(methodTy.In(i)) {
			t.Fatalf("Unexpected parameter type (idx=%d): want=%s have=%s", i, methodTy.In(i), arg.Type())
		}
		args[i] = arg
	}

	results := methodPtr.Call(args)

	// Assume that the method returns [] or [result] or [result, err]
	if len(results) == 0 {
		return nil, nil
	} else if len(results) == 1 {
		ret := results[0].Interface()
		return ret, nil
	} else if len(results) == 2 {
		ret := results[0].Interface()
		err := results[1].Interface()
		if err == nil {
			return ret, nil
		} else {
			return nil, err.(error)
		}
	} else {
		t.Fatalf("Unexpected return count: %d", len(results))
		return nil, nil
	}
}
