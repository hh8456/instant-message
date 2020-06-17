package atomic

import "sync/atomic"

type Int64 struct {
	v int64
}

func NewInt64(v int64) *Int64 {
	return &Int64{}
}

func (self *Int64) Load() int64 {
	return atomic.LoadInt64(&self.v)
}

func (self *Int64) Add(v int64) int64 {
	return atomic.AddInt64(&self.v, v)
}

func (self *Int64) Sub(v int64) int64 {
	return atomic.AddInt64(&self.v, -v)
}

func (self *Int64) Inc() int64 {
	return self.Add(1)
}

func (self *Int64) Dec() int64 {
	return self.Sub(1)
}

func (self *Int64) CAS(old, new int64) bool {
	return atomic.CompareAndSwapInt64(&self.v, old, new)
}

func (self *Int64) Store(v int64) {
	atomic.StoreInt64(&self.v, v)
}

func (self *Int64) Swap(v int64) int64 {
	return atomic.SwapInt64(&self.v, v)
}
