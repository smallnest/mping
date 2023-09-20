package main

import (
	"container/heap"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrorKeyBehind = errors.New("Key is behind")
)

type Result struct {
	txts     int64
	rxts     int64
	seq      uint16
	target   string
	latency  int64
	received bool
	bitflip  bool
}

type TargetResult struct {
	latency      int64
	loss         int
	received     int
	bitflipCount int
}

type buckets struct {
	mu sync.RWMutex
	bs queue
	m  map[int64]*Bucket
}

func NewBuckets() *buckets {
	return &buckets{
		m: make(map[int64]*Bucket),
	}
}

type queue []*Bucket

func (q queue) Len() int           { return len(q) }
func (q queue) Less(i, j int) bool { return q[i].Key < q[j].Key }
func (q queue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }

func (h *queue) Push(b any) {
	*h = append(*h, b.(*Bucket))
}

func (h *queue) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// Add adds a value to the bucket.
func (h *buckets) Add(k int64, v *Result) error {
	h.mu.Lock()
	bucket := h.m[k]
	if bucket == nil {
		bucket = &Bucket{Key: k, Value: make(map[string]*Result)}
		h.m[k] = bucket
		heap.Push(&h.bs, bucket)
	}
	h.mu.Unlock()

	bucket.Add(v)

	return nil
}

// AddReply adds a reply to the bucket.
func (h *buckets) AddReply(k int64, v *Result) error {
	h.mu.Lock()
	bucket := h.m[k]
	if bucket == nil {
		bucket = &Bucket{Key: k, Value: make(map[string]*Result)}
		h.m[k] = bucket
		heap.Push(&h.bs, bucket)
	}
	h.mu.Unlock()

	bucket.AddReply(v)

	return nil
}

// Pop returns the bucket for the key.
func (h *buckets) Pop() any {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.bs) == 0 {
		return nil
	}

	bucket := heap.Pop(&h.bs)
	if bucket == nil {
		return nil
	}

	delete(h.m, bucket.(*Bucket).Key)

	return bucket
}

// LastN returns the last n buckets but not pop them.
func (h *buckets) Last() *Bucket {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.bs) == 0 {
		return nil
	}

	buckets := h.bs[0]

	return buckets
}

// Bucket is a key-value pair.
type Bucket struct {
	Key int64

	Mu    sync.RWMutex
	Value map[string]*Result
}

// Add adds a value to the bucket.
func (b *Bucket) Add(v *Result) {
	b.Mu.Lock()
	b.Value[fmt.Sprintf("%s-%d", v.target, v.seq)] = v
	b.Mu.Unlock()
}

// AddReply adds a reply to the bucket.
func (b *Bucket) AddReply(v *Result) {
	key := fmt.Sprintf("%s-%d", v.target, v.seq)
	b.Mu.Lock()
	req := b.Value[key]
	if req == nil {
		b.Value[key] = v
	} else {
		v.latency = v.rxts - req.txts
		b.Value[key] = v
	}

	b.Mu.Unlock()
}

// Values returns the values in the bucket.
func (b *Bucket) Values() map[string]*Result {
	return b.Value
}
