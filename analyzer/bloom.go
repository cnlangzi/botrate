package analyzer

import (
	"github.com/bits-and-blooms/bloom/v3"
)

var BloomMaxCapacity uint = 100000

var BloomFalsePositiveRate = 0.01

type DoubleBufferBloom struct {
	current  *bloom.BloomFilter
	previous *bloom.BloomFilter
}

func NewDoubleBufferBloom() *DoubleBufferBloom {
	return &DoubleBufferBloom{
		current:  bloom.NewWithEstimates(BloomMaxCapacity, BloomFalsePositiveRate),
		previous: bloom.NewWithEstimates(BloomMaxCapacity, BloomFalsePositiveRate),
	}
}

func (dbf *DoubleBufferBloom) TestAndAdd(key []byte) bool {
	if dbf.current.Test(key) {
		return true
	}
	dbf.current.Add(key)
	return false
}

func (dbf *DoubleBufferBloom) Rotate() {
	newFilter := bloom.NewWithEstimates(BloomMaxCapacity, BloomFalsePositiveRate)
	dbf.previous = dbf.current
	dbf.current = newFilter
}
