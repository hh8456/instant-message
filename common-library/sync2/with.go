package sync2

import "sync"

func With(locker sync.Locker, f func())  {
	locker.Lock()
	defer locker.Unlock()
	f()
}