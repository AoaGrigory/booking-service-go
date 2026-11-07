package service

import "sync"

type StatsCacheState struct {
	mu      sync.Mutex
	version int64
}
