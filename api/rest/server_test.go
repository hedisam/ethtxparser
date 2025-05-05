package rest_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	restapi "github.com/hedisam/ethtxparser/api/rest"
	"github.com/hedisam/ethtxparser/api/rest/mocks"
	"github.com/hedisam/ethtxparser/internal/store"
)

//go:generate moq -out mocks/tx_store.go -pkg mocks -skip-ensure . TxStore
//go:generate moq -out mocks/subscriptions_store.go -pkg mocks -skip-ensure . SubscriptionStore

func TestGetCurrentBlock(t *testing.T) {
	tests := map[string]struct {
		req                *restapi.GetCurrentBlockRequest
		currentBlockNumber *int64
		expectedStoreCalls int
		expectedResp       *restapi.GetCurrentBlockResponse
		expectedErr        *restapi.Err
	}{
		"no blocks yet": {
			currentBlockNumber: nil,
			expectedStoreCalls: 1,
			expectedErr: &restapi.Err{
				Message:    "No parsed blocks yet, please retry later",
				StatusCode: http.StatusServiceUnavailable,
			},
		},
		"success": {
			currentBlockNumber: ptr[int64](1234),
			expectedStoreCalls: 1,
			expectedResp: &restapi.GetCurrentBlockResponse{
				BlockNumber:    "0x4d2",
				BlockNumberInt: int64(1234),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			storeMock := &mocks.TxStoreMock{
				GetCurrentBlockNumberFunc: func(ctx context.Context) (int64, error) {
					if test.currentBlockNumber == nil {
						return 0, store.ErrNotFound
					}
					return *test.currentBlockNumber, nil
				},
			}
			s := restapi.NewServer(logrus.New(), storeMock, nil)
			resp, err := s.GetCurrentBlock(context.Background(), test.req)
			assert.Equal(t, test.expectedStoreCalls, len(storeMock.GetCurrentBlockNumberCalls()))
			if test.expectedErr != nil {
				require.Error(t, err)
				castedErr := &restapi.Err{}
				if errors.As(err, &castedErr) {
					assert.Equal(t, test.expectedErr, castedErr)
					return
				}
				assert.Equal(t, test.expectedErr.Message, err.Error())
				return
			}

			assert.Equal(t, test.expectedResp, resp)
		})
	}
}

func TestSubscribe(t *testing.T) {
	tests := map[string]struct {
		req                *restapi.SubscribeRequest
		storeErr           error
		expectedStoreCalls int
		expectedResp       *restapi.SubscribeResponse
		expectedErr        *restapi.Err
	}{
		"valid address": {
			req: &restapi.SubscribeRequest{
				Address: "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
			},
			expectedStoreCalls: 1,
			expectedResp: &restapi.SubscribeResponse{
				Ok: true,
			},
		},
		"empty address": {
			req: &restapi.SubscribeRequest{
				Address: "",
			},
			expectedErr: &restapi.Err{
				StatusCode: http.StatusBadRequest,
				Message:    "Missing required field: 'address'",
			},
		},
		"too short address": {
			req: &restapi.SubscribeRequest{
				Address: "0x1234",
			},
			expectedErr: &restapi.Err{
				StatusCode: http.StatusBadRequest,
				Message:    restapi.InvalidAddrMessage,
			},
		},
		"invalid hex address": {
			req: &restapi.SubscribeRequest{
				Address: "0xZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ",
			},
			expectedErr: &restapi.Err{
				StatusCode: http.StatusBadRequest,
				Message:    restapi.InvalidAddrMessage,
			},
		},
		"store failure": {
			req: &restapi.SubscribeRequest{
				Address: "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
			},
			expectedStoreCalls: 1,
			storeErr:           errors.New("dummy error"),
			expectedErr: &restapi.Err{
				StatusCode: http.StatusInternalServerError,
				Message:    "could not add address subscription to store",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			storeMock := &mocks.SubscriptionStoreMock{
				AddSubscriptionFunc: func(ctx context.Context, addr string) error {
					return test.storeErr
				},
			}
			s := restapi.NewServer(logrus.New(), nil, storeMock)
			resp, err := s.Subscribe(context.Background(), test.req)
			assert.Equal(t, test.expectedStoreCalls, len(storeMock.AddSubscriptionCalls()))
			if test.expectedErr != nil {
				require.Error(t, err)
				castedErr := &restapi.Err{}
				if errors.As(err, &castedErr) {
					assert.Equal(t, test.expectedErr, castedErr)
					return
				}
				assert.Equal(t, test.expectedErr.Message, err.Error())
				return
			}

			assert.Equal(t, test.expectedResp, resp)
		})
	}
}

func TestGetTransactions(t *testing.T) {
	tests := map[string]struct {
		req                               *restapi.ListTransactionsRequest
		storeErr                          error
		storeResp                         []*store.TxRecord
		subscribedAddresses               []string
		expectedStoreGetTransactionsCalls int
		expectedStoreIsSubscribedCalls    int
		expectedResp                      *restapi.ListTransactionsResponse
		expectedErr                       *restapi.Err
	}{
		"success": {
			req: &restapi.ListTransactionsRequest{
				Address: "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
			},
			subscribedAddresses: []string{"0x7a250d5630b4cf539739df2c5dacb4c659f2488d"},
			storeResp: []*store.TxRecord{
				{
					Hash:        "hash-1",
					From:        "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
					To:          "to-1",
					BlockNumber: 1,
					BlockHash:   "block-hash-1",
					Raw:         []byte(`{"key": "value-1"}`),
				},
				{
					Hash:        "hash-2",
					From:        "from-2",
					To:          "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
					BlockNumber: 2,
					BlockHash:   "block-hash-2",
					Raw:         []byte(`{"key": "value-2"}`),
				},
			},
			expectedStoreGetTransactionsCalls: 1,
			expectedStoreIsSubscribedCalls:    1,
			expectedResp: &restapi.ListTransactionsResponse{
				Transactions: []*restapi.Transaction{
					{
						Hash:           "hash-1",
						From:           "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
						To:             "to-1",
						BlockNumber:    "0x1",
						BlockNumberInt: 1,
						BlockHash:      "block-hash-1",
						FullTx:         map[string]any{"key": "value-1"},
					},
					{
						Hash:           "hash-2",
						From:           "from-2",
						To:             "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
						BlockNumber:    "0x2",
						BlockNumberInt: 2,
						BlockHash:      "block-hash-2",
						FullTx:         map[string]any{"key": "value-2"},
					},
				},
			},
		},
		"empty address": {
			req: &restapi.ListTransactionsRequest{
				Address: " ",
			},
			expectedErr: &restapi.Err{
				StatusCode: http.StatusBadRequest,
				Message:    "Missing required field: 'address'",
			},
		},
		"invalid hex address": {
			req: &restapi.ListTransactionsRequest{
				Address: "0xZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ",
			},
			expectedErr: &restapi.Err{
				StatusCode: http.StatusBadRequest,
				Message:    restapi.InvalidAddrMessage,
			},
		},
		"store failure": {
			req: &restapi.ListTransactionsRequest{
				Address: "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
			},
			expectedStoreGetTransactionsCalls: 1,
			expectedStoreIsSubscribedCalls:    1,
			subscribedAddresses:               []string{"0x7a250d5630b4cf539739df2c5dacb4c659f2488d"},
			storeErr:                          errors.New("dummy error"),
			expectedErr: &restapi.Err{
				StatusCode: http.StatusInternalServerError,
				Message:    "Could not list transactions from store",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			txStoreMock := &mocks.TxStoreMock{
				GetTransactionsFunc: func(ctx context.Context, addr string) ([]*store.TxRecord, error) {
					assert.Equal(t, test.req.Address, addr)
					return test.storeResp, test.storeErr
				},
			}
			subsStoreMock := &mocks.SubscriptionStoreMock{
				IsSubscribedFunc: func(ctx context.Context, addr string) (bool, error) {
					assert.Equal(t, test.req.Address, addr)
					ok := slices.Contains(test.subscribedAddresses, addr)
					return ok, nil
				},
			}
			s := restapi.NewServer(logrus.New(), txStoreMock, subsStoreMock)
			resp, err := s.ListTransactions(context.Background(), test.req)
			assert.Equal(t, test.expectedStoreGetTransactionsCalls, len(txStoreMock.GetTransactionsCalls()))
			assert.Equal(t, test.expectedStoreIsSubscribedCalls, len(subsStoreMock.IsSubscribedCalls()))
			if test.expectedErr != nil {
				require.Error(t, err)
				castedErr := &restapi.Err{}
				if errors.As(err, &castedErr) {
					assert.Equal(t, test.expectedErr, castedErr)
					return
				}
				assert.Equal(t, test.expectedErr.Message, err.Error())
				return
			}
			require.NoError(t, err)

			require.NotNil(t, resp)
			assert.Equal(t, len(test.expectedResp.Transactions), len(resp.Transactions))
			for i, expected := range test.expectedResp.Transactions {
				assert.Equal(t, expected, resp.Transactions[i])
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
