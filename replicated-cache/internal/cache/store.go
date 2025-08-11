/*
Author: phyu lwin
Project: Replicated In-Memory Cache (Golang)
Date: Aug 10th 2025

Summary:
This file implements a concurrent, in-memory Last-Write-Wins (LWW) map for use as a replicated cache store.
It provides thread-safe methods for storing, retrieving, and expiring cache items, supporting versioning and tombstone-based deletion.

Functions:
- NewStore(): *Store
- (*Store) Get(key string): (Item, bool)
- (*Store) Put(key string, incoming Item): bool
- (*Store) HardDeleteExpired(now time.Time, tombstoneTTL time.Duration)
*/

package cache

import (
	"sync"
	"time"
)

// Store is a concurrent, in-memory LWW map.
type Store struct {
	mu   sync.RWMutex
	data map[string]Item
}

func NewStore() *Store { return &Store{data: make(map[string]Item)} }

func (s *Store) Get(key string) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.data[key]
	return it, ok
}

// Put applies last-write-wins using Version (then Origin to break ties).
func (s *Store) Put(key string, incoming Item) (applied bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, exists := s.data[key]
	if !exists {
		s.data[key] = incoming
		return true
	}
	if incoming.Version > cur.Version || (incoming.Version == cur.Version && incoming.Origin > cur.Origin) {
		s.data[key] = incoming
		return true
	}
	return false
}

func (s *Store) HardDeleteExpired(now time.Time, tombstoneTTL time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.data {
		if v.Tombstone && now.Sub(time.Unix(0, v.Version)) > tombstoneTTL {
			delete(s.data, k)
			continue
		}
		if !v.Tombstone && v.expired(now) {
			delete(s.data, k)
		}
	}
}