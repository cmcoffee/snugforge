package xsync

import (
	"sync/atomic"
)

// Atomic BitFlag
type BitFlag uint64

// Check if flag is set
func (B *BitFlag) Has(flag uint64) bool {
	if atomic.LoadUint64((*uint64)(B))&uint64(flag) != 0 {
		return true
	}
	return false
}

// Set BitFlag
func (B *BitFlag) Set(flag uint64) {
	for {
		old := atomic.LoadUint64((*uint64)(B))
		if atomic.CompareAndSwapUint64((*uint64)(B), old, old|uint64(flag)) {
			return
		}
	}
}

// Unset BitFlag
func (B *BitFlag) Unset(flag uint64) {
	for {
		old := atomic.LoadUint64((*uint64)(B))
		if atomic.CompareAndSwapUint64((*uint64)(B), old, old&^uint64(flag)) {
			return
		}
	}
}

// Perform lookup of multiple flags, return index of first match or 0 if none
func (B *BitFlag) Switch(flags ...uint64) uint64 {
	for _, flag := range flags {
		if B.Has(flag) {
			return flag
		}
	}
	return 0
}
