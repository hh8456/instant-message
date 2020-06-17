package atomic

import "sync/atomic"

type Bool struct {
	v int32
}

//NewBool creates an Bool with given default value
func NewBool(v bool) *Bool {
	ab := &Bool{}
	if v {
		ab.Set()
	}
	return ab
}

// Set sets the Boolean to true
func (self *Bool) Set()  {
	atomic.StoreInt32(&self.v, 1)
}

// UnSet set the Boolean to false
func (self *Bool) UnSet()  {
	atomic.StoreInt32(&self.v, 0)
}

// IsSet returns whether the Boolean is true
func (self *Bool) IsSet() bool  {
	return atomic.LoadInt32(&self.v) == 1
}

//SetTo sets the Boolean with given Boolean
func (self *Bool) SetTo(yes bool)  {
	if yes {
		self.Set()
	} else {
		self.UnSet()
	}
}

//SetToIf sets the Boolean to new only if the Boolean matches the old
// Returns whether the set was done
func (self *Bool)  SetToIf(old, new bool) (set bool) {
	var o, n int32
	if old {
		o = 1
	}
	if new {
		n = 1
	}
	return atomic.CompareAndSwapInt32(&self.v, o, n)
}
