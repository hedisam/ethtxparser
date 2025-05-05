package rest

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/hedisam/ethtxparser/internal/store"
)

const (
	// InvalidAddrMessage is returned when users make a request with an invalid addr.
	InvalidAddrMessage = "Invalid Ethereum address. Expected a 40-character hex string, with or without '0x' prefix. Example: 0x12ab34cd56ef7890a1234567890abcdef1234567"
)

type TxStore interface {
	GetCurrentBlockNumber(ctx context.Context) (int64, error)
	GetTransactions(ctx context.Context, addr string) ([]*store.TxRecord, error)
}

type SubscriptionStore interface {
	AddSubscription(ctx context.Context, addr string) error
	GetSubscriptions(ctx context.Context) ([]string, error)
	IsSubscribed(ctx context.Context, addr string) (bool, error)
}

type Server struct {
	logger    *logrus.Logger
	txStore   TxStore
	subsStore SubscriptionStore
}

func NewServer(logger *logrus.Logger, txStore TxStore, subsStore SubscriptionStore) *Server {
	return &Server{
		logger:    logger,
		txStore:   txStore,
		subsStore: subsStore,
	}
}

func (s *Server) GetCurrentBlock(ctx context.Context, _ *GetCurrentBlockRequest) (*GetCurrentBlockResponse, error) {
	logger := s.logger.WithContext(ctx)

	blockNumber, err := s.txStore.GetCurrentBlockNumber(ctx)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			logger.Warn("No parsed blocks yet when requesting current block number")
			return nil, NewErrf(http.StatusServiceUnavailable, "No parsed blocks yet, please retry later")
		}
		logger.WithError(err).Error("Failed to get current block number from store")
		return nil, NewErrf(http.StatusInternalServerError, "could not get current block number from store")
	}

	return &GetCurrentBlockResponse{
		BlockNumberInt: blockNumber,
		BlockNumber:    fmt.Sprintf("0x%x", blockNumber),
	}, nil
}

func (s *Server) Subscribe(ctx context.Context, req *SubscribeRequest) (*SubscribeResponse, error) {
	logger := s.logger.WithContext(ctx).WithField("addr", req.Address)

	addr := strings.TrimSpace(req.Address)
	if addr == "" {
		logger.Warn("Address is required to subscribe to")
		return nil, NewErrf(http.StatusBadRequest, "Missing required field: 'address'")
	}

	addr, valid := validateAndNormalizeAddress(addr)
	if !valid {
		logger.Warn("Invalid address provided to subscribe to")
		return nil, NewErrf(http.StatusBadRequest, InvalidAddrMessage)
	}

	err := s.subsStore.AddSubscription(ctx, addr)
	if err != nil {
		logger.WithError(err).Error("Failed to add address subscription to store")
		return nil, NewErrf(http.StatusInternalServerError, "could not add address subscription to store")
	}

	return &SubscribeResponse{
		Ok: true,
	}, nil
}

func (s *Server) ListSubscriptions(ctx context.Context, _ *ListSubscriptionRequest) (*ListSubscriptionResponse, error) {
	logger := s.logger.WithContext(ctx)

	addresses, err := s.subsStore.GetSubscriptions(ctx)
	if err != nil {
		logger.WithError(err).Error("Failed to list subscribed addresses from store")
		return nil, NewErrf(http.StatusInternalServerError, "could not list subscribed addresses")
	}

	return &ListSubscriptionResponse{
		Addresses: addresses,
	}, nil
}

func (s *Server) ListTransactions(ctx context.Context, req *ListTransactionsRequest) (*ListTransactionsResponse, error) {
	logger := s.logger.WithContext(ctx).WithField("addr", req.Address)

	addr := strings.TrimSpace(req.Address)
	if addr == "" {
		logger.Warn("Address is required to list transactions")
		return nil, NewErrf(http.StatusBadRequest, "Missing required field: 'address'")
	}

	addr, valid := validateAndNormalizeAddress(addr)
	if !valid {
		logger.Warn("Invalid address provided to list transactions")
		return nil, NewErrf(http.StatusBadRequest, InvalidAddrMessage)
	}

	ok, err := s.subsStore.IsSubscribed(ctx, addr)
	if err != nil {
		logger.WithError(err).Error("Failed to check address subscription status while listing transactions")
		return nil, NewErrf(http.StatusInternalServerError, "Could not check address subscription status")
	}
	if !ok {
		logger.Warn("Cannot get transactions for an address not subscribed")
		return nil, NewErrf(http.StatusNotFound, "Address not subscribed. You must first subscribe to the requested address to record and retrieve its transactions.")
	}

	storedTransactions, err := s.txStore.GetTransactions(ctx, req.Address)
	if err != nil {
		logger.WithError(err).Error("Failed to get transactions from store")
		return nil, NewErrf(http.StatusInternalServerError, "Could not list transactions from store")
	}

	var txs []*Transaction
	for storedTx := range slices.Values(storedTransactions) {
		tx, err := convertStoredToAPITransaction(storedTx)
		if err != nil {
			logger.WithError(err).Error("Failed to unmarshal transaction in ListTransactions")
			return nil, NewErrf(http.StatusInternalServerError, "Could not unmarshal transaction")
		}

		txs = append(txs, tx)
	}

	return &ListTransactionsResponse{
		Transactions: txs,
	}, nil
}

func validateAndNormalizeAddress(addr string) (string, bool) {
	addr = strings.ToLower(strings.TrimSpace(addr))
	addr = strings.TrimPrefix(addr, "0x")
	if len(addr) != 40 {
		return "", false
	}

	_, err := hex.DecodeString(addr)
	if err != nil {
		return "", false
	}

	addr = "0x" + addr
	return addr, true
}

func convertStoredToAPITransaction(tx *store.TxRecord) (*Transaction, error) {
	var fullTx map[string]any
	err := json.Unmarshal(tx.Raw, &fullTx)
	if err != nil {
		return nil, fmt.Errorf("unmarshal full stored transaction: %w", err)
	}

	return &Transaction{
		Hash:           tx.Hash,
		From:           tx.From,
		To:             tx.To,
		BlockNumber:    fmt.Sprintf("0x%x", tx.BlockNumber),
		BlockNumberInt: tx.BlockNumber,
		BlockHash:      tx.BlockHash,
		FullTx:         fullTx,
	}, nil
}
