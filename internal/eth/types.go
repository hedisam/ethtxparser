package eth

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type rpcMethod string

// ID returns the ID associated with the rpc method used in json-rpc requests.
func (rm rpcMethod) ID() int {
	switch rm {
	case getCurrentBlockNumber:
		return 1
	case getBlockByNumberID:
		return 2
	default:
		return -1
	}
}

type Block struct {
	Hash       string `json:"hash"`
	Number     int64  `json:"number"`
	ParentHash string `json:"parentHash"`
	Txs        []*Tx  `json:"transactions"`
}

// UnmarshalJSON customizes Block decoding to parse the hex block number.
func (b *Block) UnmarshalJSON(data []byte) error {
	// alias to avoid infinite recursion
	type blockAlias Block
	aux := &struct {
		*blockAlias
		Number string `json:"number"`
	}{
		blockAlias: (*blockAlias)(b),
	}

	err := json.Unmarshal(data, &aux)
	if err != nil {
		return fmt.Errorf("error unmarshalling Block: %w", err)
	}

	blockNumStr := strings.TrimPrefix(aux.Number, "0x")
	blockNum, err := strconv.ParseInt(blockNumStr, 16, 64)
	if err != nil {
		return fmt.Errorf("invalid block number %q: %w", aux.Number, err)
	}
	b.Number = blockNum

	return nil
}

type Tx struct {
	Hash string `json:"hash"`
	From string `json:"from"`
	To   string `json:"to"`
	Raw  []byte `json:"-"`
}

// UnmarshalJSON ensures Hash, From, and To are parsed and the full raw JSON is stored.
func (t *Tx) UnmarshalJSON(data []byte) error {
	var aux struct {
		Hash string `json:"hash"`
		From string `json:"from"`
		To   string `json:"to"`
	}
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return fmt.Errorf("unmarshal into aux tx: %w", err)
	}

	t.Hash = aux.Hash
	t.From = aux.From
	t.To = aux.To
	t.Raw = append([]byte(nil), data...) // make a copy; safe against mutations

	return nil
}
