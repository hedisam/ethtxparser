package rest

// request and response types are defined below
// these types can be defined as protobuf messages in a production system (specifically if using gRPC + gRPC-gateway)

type GetCurrentBlockRequest struct{}

type GetCurrentBlockResponse struct {
	BlockNumber    string `json:"blockNumber"`
	BlockNumberInt int64  `json:"blockNumberInt"`
}

type SubscribeRequest struct {
	Address string `json:"address"`
}

type SubscribeResponse struct {
	Ok bool `json:"ok"`
}

type ListSubscriptionRequest struct{}

type ListSubscriptionResponse struct {
	Addresses []string `json:"addresses"`
}

type ListTransactionsRequest struct {
	Address string `json:"address"`
}

type ListTransactionsResponse struct {
	Transactions []*Transaction `json:"transactions"`
}

type Transaction struct {
	Hash           string         `json:"hash,omitempty"`
	From           string         `json:"from,omitempty"`
	To             string         `json:"to,omitempty"`
	BlockNumber    string         `json:"blockNumber,omitempty"`
	BlockNumberInt int64          `json:"blockNumberInt,omitempty"`
	BlockHash      string         `json:"blockHash,omitempty"`
	FullTx         map[string]any `json:"fullTx,omitempty"`
}
