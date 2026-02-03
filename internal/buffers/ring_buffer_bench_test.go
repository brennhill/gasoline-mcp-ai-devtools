package buffers

import (
	"testing"
	"time"
)

// BenchmarkRingBufferWriteOne measures single entry write performance
func BenchmarkRingBufferWriteOne(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.WriteOne(i)
	}
}

// BenchmarkRingBufferWrite measures batch write performance
func BenchmarkRingBufferWrite(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	batch := make([]int, 10)
	for i := range batch {
		batch[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write(batch)
	}
}

// BenchmarkRingBufferReadFrom measures cursor read performance
func BenchmarkRingBufferReadFrom(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	// Pre-populate buffer
	batch := make([]int, 100)
	for i := 0; i < 10; i++ {
		rb.Write(batch)
	}
	cursor := BufferCursor{Position: 500, Timestamp: time.Now()}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.ReadFrom(cursor)
	}
}

// BenchmarkRingBufferReadAll measures full buffer read performance
func BenchmarkRingBufferReadAll(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	// Fill buffer to capacity
	batch := make([]int, 1000)
	rb.Write(batch)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.ReadAll()
	}
}

// BenchmarkRingBufferWriteWithEviction measures write with eviction overhead
func BenchmarkRingBufferWriteWithEviction(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	// Fill buffer to capacity
	batch := make([]int, 1000)
	rb.Write(batch)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.WriteOne(i) // This will trigger eviction
	}
}

// BenchmarkRingBufferConcurrent measures concurrent read/write performance
func BenchmarkRingBufferConcurrent(b *testing.B) {
	rb := NewRingBuffer[int](10000)
	cursor := BufferCursor{Position: 0, Timestamp: time.Now()}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				rb.WriteOne(i)
			} else {
				rb.ReadFrom(cursor)
			}
			i++
		}
	})
}
