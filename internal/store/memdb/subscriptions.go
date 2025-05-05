package memdb

import (
	"context"
	"maps"
	"slices"
	"sync"
)

// SubscriptionStore keeps a record of subscribed addresses.
type SubscriptionStore struct {
	subscribedAddresses map[string]struct{}
	mu                  sync.RWMutex
}

func NewSubscriptionStore(opts ...Option) *SubscriptionStore {
	cfg := &config{memSize: DefaultMemSize}
	for opt := range slices.Values(opts) {
		opt(cfg)
	}

	return &SubscriptionStore{
		subscribedAddresses: make(map[string]struct{}, cfg.memSize),
	}
}

// AddSubscription adds a new address to the list of subscribed addresses.
// Nothing happens if we've already subscribed to the specified address.
func (s *SubscriptionStore) AddSubscription(_ context.Context, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribedAddresses[addr] = struct{}{}
	return nil
}

// IsSubscribed returns true if we have subscribed to the given address.
func (s *SubscriptionStore) IsSubscribed(_ context.Context, addr string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.subscribedAddresses[addr]
	return ok, nil
}

// GetSubscriptions returns the currently subscribed addresses.
func (s *SubscriptionStore) GetSubscriptions(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Collect(maps.Keys(s.subscribedAddresses)), nil
}
