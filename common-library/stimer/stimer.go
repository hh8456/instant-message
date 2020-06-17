package stimer

import (
	"sync"
	"time"

	"instant-message/common-library/log"
	"instant-message/common-library/sync2"
	"instant-message/common-library/sync2/atomic"
	"instant-message/common-library/timer"
)

type sTimer struct {
	globalId       atomic.Int64
	timersMap      map[int64]timer.TimerId
	timersMapMutex *sync.Mutex
}

var instance *sTimer
var instanceOnce sync.Once

func Instance() *sTimer {
	instanceOnce.Do(func() {
		instance = &sTimer{
			timersMap:      make(map[int64]timer.TimerId),
			timersMapMutex: &sync.Mutex{},
		}
	})
	return instance
}

func (self *sTimer) RunAfter(d time.Duration, description string, action func()) int64 {
	id := self.globalId.Inc()
	timerId := timer.RunAfter(d, description, func() {
		action()
		sync2.With(self.timersMapMutex, func() {
			if v, ok := self.timersMap[id]; ok {
				v.Stop()
			} else {
				log.Error("stimer.go, sTimer.timersMap 中没找到对应的定时器")
			}

			delete(self.timersMap, id)
		})
	})

	sync2.With(self.timersMapMutex, func() {
		self.timersMap[id] = timerId
	})
	return id
}

func (self *sTimer) RunEver(interval time.Duration, description string, action func()) int64 {
	id := self.globalId.Inc()
	timerId := timer.RunEvery(interval, description, action)

	sync2.With(self.timersMapMutex, func() {
		self.timersMap[id] = timerId
	})
	return id
}

// cronExpr 格式参考 https://en.wikipedia.org/wiki/Cron
// Field name   | Mandatory? | Allowed values | Allowed special characters
// ----------   | ---------- | -------------- | --------------------------
// Seconds      | No         | 0-59           | * / , -
// Minutes      | Yes        | 0-59           | * / , -
// Hours        | Yes        | 0-23           | * / , -
// Day of month | Yes        | 1-31           | * / , -
// Month        | Yes        | 1-12           | * / , -
// Day of week  | Yes        | 0-6            | * / , -
func (self *sTimer) Cron(cronExprStr string, description string, action func()) int64 {
	id := self.globalId.Inc()
	timerId := timer.Cron(cronExprStr, description, action)

	sync2.With(self.timersMapMutex, func() {
		self.timersMap[id] = timerId
	})
	return id
}

// CustomCron 自定义的定时任务
// cronExpStr 2018,8,20,22,22,22,5 最后一位为星期;
func (self *sTimer) CustomCron(cronExprStr string, description string, action func()) int64 {
	id := self.globalId.Inc()
	timerId := timer.Customcron(cronExprStr, description, action)

	sync2.With(self.timersMapMutex, func() {
		self.timersMap[id] = timerId
	})
	return id
}

func (self *sTimer) StopTimer(id int64) {
	var timeId timer.TimerId

	sync2.With(self.timersMapMutex, func() {
		timeId = self.timersMap[id]
		delete(self.timersMap, id)
	})

	if timeId != nil {
		timeId.Stop()
	}
}
