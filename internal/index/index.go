package index

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/hedisam/ethtxparser/internal/eth"
	"github.com/hedisam/ethtxparser/internal/store"
	"github.com/hedisam/pipeline/chans"
)

type SubscriptionStore interface {
	IsSubscribed(ctx context.Context, addr string) (bool, error)
}

type TxStore interface {
	InsertBlock(ctx context.Context, block *store.Block) error
}

type Index struct {
	logger            *logrus.Logger
	txStore           TxStore
	subscriptionStore SubscriptionStore
}

func New(logger *logrus.Logger, txStore TxStore, subscriptionStore SubscriptionStore) *Index {
	return &Index{
		logger:            logger,
		txStore:           txStore,
		subscriptionStore: subscriptionStore,
	}
}

func (i *Index) Start(ctx context.Context, in <-chan *eth.Block) {
	for block := range chans.ReceiveOrDoneSeq(ctx, in) {
		err := i.index(ctx, block)
		if err != nil {
			i.logger.WithFields(logrus.Fields{
				"block_hash":   block.Hash,
				"block_number": block.Number,
			}).WithError(err).Error("Failed to index block")
			blocksFailedProcessing.Inc()
		}
	}
}

func (i *Index) index(ctx context.Context, block *eth.Block) error {
	if block == nil {
		return nil
	}

	logger := i.logger.WithContext(ctx).WithFields(logrus.Fields{
		"block_number": block.Number,
		"total_txs":    len(block.Txs),
	})

	addrToTxs := make(map[string][]*store.TxRecord, len(block.Txs))
	var totalIndexedTxs int
	for tx := range slices.Values(block.Txs) {
		subscribedAddresses, err := i.subscribedAddresses(ctx, tx)
		if err != nil {
			return fmt.Errorf("could not check for subscribed addresses for tx %q: %w", tx.Hash, err)
		}
		for addr := range slices.Values(subscribedAddresses) {
			addrToTxs[addr] = append(addrToTxs[addr], &store.TxRecord{
				Hash:        tx.Hash,
				From:        tx.From,
				To:          tx.To,
				BlockNumber: block.Number,
				BlockHash:   block.Hash,
				Raw:         tx.Raw,
			})
		}
		if len(subscribedAddresses) > 0 {
			totalIndexedTxs++
		}
	}

	err := i.txStore.InsertBlock(ctx, &store.Block{
		Number:     block.Number,
		Hash:       block.Hash,
		ParentHash: block.ParentHash,
		AddrToTxs:  addrToTxs,
	})
	if err != nil {
		return fmt.Errorf("could not insert block into store: %w", err)
	}

	processedBlocks.Inc()
	indexedTransactions.Add(float64(totalIndexedTxs))

	logger.WithField("indexed_txs", totalIndexedTxs).Debug("Successfully processed block")

	return nil
}

func (i *Index) subscribedAddresses(ctx context.Context, tx *eth.Tx) ([]string, error) {
	var subscribedAddresses []string
	for addr := range slices.Values([]string{tx.To, tx.From}) {
		ok, err := i.subscriptionStore.IsSubscribed(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("could not check subscription existence for tx addr %q: %w", addr, err)
		}
		if ok {
			subscribedAddresses = append(subscribedAddresses, strings.ToLower(addr))
		}
	}

	return subscribedAddresses, nil
}
