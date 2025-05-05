package memdb

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/hedisam/ethtxparser/internal/store"
)

const (
	// BlockNone is used to denote we haven't processed any blocks yet.
	BlockNone = -1
)

// TxStore holds a record of parsed and indexed transactions for the subscribed addresses.
type TxStore struct {
	addrToTransactions map[string][]*store.TxRecord
	currentBlockNum    *atomic.Int64
	mu                 sync.RWMutex
}

func NewTxStore(opts ...Option) *TxStore {
	cfg := &config{memSize: DefaultMemSize}
	for opt := range slices.Values(opts) {
		opt(cfg)
	}

	var currentBlockNum atomic.Int64
	currentBlockNum.Store(BlockNone)
	return &TxStore{
		addrToTransactions: make(map[string][]*store.TxRecord, cfg.memSize),
		currentBlockNum:    &currentBlockNum,
	}
}

// InsertBlock inserts block and transactions details within a single db transaction.
func (s *TxStore) InsertBlock(_ context.Context, block *store.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentBlockNum.Store(block.Number)
	for addr, txs := range block.AddrToTxs {
		s.addrToTransactions[addr] = append(s.addrToTransactions[addr], txs...)
	}

	return nil
}

// GetTransactions returns recorded transactions for the given addr.
func (s *TxStore) GetTransactions(_ context.Context, addr string) ([]*store.TxRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.addrToTransactions[addr], nil
}

// GetCurrentBlockNumber returns the last parsed block number.
func (s *TxStore) GetCurrentBlockNumber(_ context.Context) (int64, error) {
	blockNum := s.currentBlockNum.Load()
	if blockNum == BlockNone {
		return BlockNone, store.ErrNotFound
	}

	return blockNum, nil
}
