package index

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hedisam/ethtxparser/internal/eth"
	"github.com/hedisam/ethtxparser/internal/index/mocks"
	"github.com/hedisam/ethtxparser/internal/store"
)

//go:generate moq -out mocks/tx_store.go -pkg mocks -skip-ensure . TxStore
//go:generate moq -out mocks/subscriptions_store.go -pkg mocks -skip-ensure . SubscriptionStore

func TestIndex(t *testing.T) {
	tests := map[string]struct {
		block                          *eth.Block
		subscribedAddresses            []string
		storeInsertErr                 error
		expectedStoreIsSubscribedCalls int
		expectedStoreInsertCalls       int
		expectedIndexedBlock           *store.Block
		errContains                    string
	}{
		"block with subscribed addresses": {
			block: &eth.Block{
				Hash:       "hash-1",
				Number:     1,
				ParentHash: "0x0",
				Txs: []*eth.Tx{
					{
						Hash: "tx-1",
						From: "addr-1",
						To:   "addr-2",
						Raw:  []byte("raw-1"),
					},
					{
						Hash: "tx-2",
						From: "addr-10",
						To:   "addr-1",
						Raw:  []byte("raw-1"),
					},
					{
						Hash: "tx-3",
						From: "addr-11",
						To:   "addr-3",
						Raw:  []byte("raw-1"),
					},
					{
						Hash: "tx-4",
						From: "addr-11",
						To:   "addr-4",
						Raw:  []byte("raw-1"),
					},
				},
			},
			subscribedAddresses:            []string{"addr-1", "addr-3"},
			expectedStoreInsertCalls:       1,
			expectedStoreIsSubscribedCalls: 8,
			expectedIndexedBlock: &store.Block{
				Number:     1,
				Hash:       "hash-1",
				ParentHash: "0x0",
				AddrToTxs: map[string][]*store.TxRecord{
					"addr-1": {
						{
							Hash:        "tx-1",
							From:        "addr-1",
							To:          "addr-2",
							BlockNumber: 1,
							BlockHash:   "hash-1",
							Raw:         []byte("raw-1"),
						},
						{
							Hash:        "tx-2",
							From:        "addr-10",
							To:          "addr-1",
							BlockNumber: 1,
							BlockHash:   "hash-1",
							Raw:         []byte("raw-1"),
						},
					},
					"addr-3": {
						{
							Hash:        "tx-3",
							From:        "addr-11",
							To:          "addr-3",
							BlockNumber: 1,
							BlockHash:   "hash-1",
							Raw:         []byte("raw-1"),
						},
					},
				},
			},
		},
		"block with no transactions": {
			block: &eth.Block{
				Hash:       "hash-1",
				Number:     1,
				ParentHash: "0x0",
				Txs:        nil,
			},
			subscribedAddresses:            []string{"addr-1", "addr-3"},
			expectedStoreInsertCalls:       1,
			expectedStoreIsSubscribedCalls: 0,
			expectedIndexedBlock: &store.Block{
				Number:     1,
				Hash:       "hash-1",
				ParentHash: "0x0",
				AddrToTxs:  map[string][]*store.TxRecord{},
			},
		},
		"store error": {
			block: &eth.Block{
				Hash:       "hash-1",
				Number:     1,
				ParentHash: "0x0",
				Txs: []*eth.Tx{
					{
						Hash: "tx-1",
						From: "addr-1",
						To:   "addr-2",
						Raw:  []byte("raw-1"),
					},
				},
			},
			subscribedAddresses:            []string{"addr-1", "addr-3"},
			expectedStoreInsertCalls:       1,
			expectedStoreIsSubscribedCalls: 2,
			storeInsertErr:                 errors.New("internal error"),
			errContains:                    "internal error",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			txStoreMock := &mocks.TxStoreMock{
				InsertBlockFunc: func(ctx context.Context, block *store.Block) error {
					for addr := range block.AddrToTxs {
						assert.Contains(t, test.subscribedAddresses, addr)
					}
					return test.storeInsertErr
				},
			}
			subsStoreMock := &mocks.SubscriptionStoreMock{
				IsSubscribedFunc: func(ctx context.Context, addr string) (bool, error) {
					return slices.Contains(test.subscribedAddresses, addr), nil
				},
			}

			idx := New(logrus.New(), txStoreMock, subsStoreMock)
			err := idx.index(context.Background(), test.block)
			assert.Equal(t, test.expectedStoreInsertCalls, len(txStoreMock.InsertBlockCalls()))
			assert.Equal(t, test.expectedStoreIsSubscribedCalls, len(subsStoreMock.IsSubscribedCalls()))
			if test.errContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, test.errContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expectedIndexedBlock, txStoreMock.InsertBlockCalls()[0].Block)
		})
	}
}
