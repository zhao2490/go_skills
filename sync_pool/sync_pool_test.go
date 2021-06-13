package sync_pool

import (
	"math/rand"
	"testing"
)

func BenchmarkSyncPool(b *testing.B) {
	pool := NewPool()
	for i := 0; i < b.N; i++ {
		size := rand.Intn(19999)
		size++
		if size == 0 {
			panic("nil size")
		}
		s := pool.Alloc(size)
		s = append(s, 1)
		pool.Free(s)
	}
}

func BenchmarkNoSyncPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		size := rand.Intn(19999)
		size++
		s := make([]int64, size)
		s = append(s, 1)
	}
}
