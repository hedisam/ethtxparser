package store

import "errors"

var (
	// ErrNotFound is returned when an item in store is not found.
	ErrNotFound = errors.New("not found")
)

type TxRecord struct {
	Hash        string `json:"hash"`
	From        string `json:"from"`
	To          string `json:"to"`
	BlockNumber int64  `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
	Raw         []byte `json:"-"`
}

type Block struct {
	Number     int64
	Hash       string
	ParentHash string
	AddrToTxs  map[string][]*TxRecord
}
