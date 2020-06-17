package timer

import (
	"sync"
	"time"

	"instant-message/common-library/log"
	"instant-message/common-library/sync2"
	"instant-message/common-library/sync2/atomic"
)

type TimerId interface {
	Description() string
	Active() bool
	Stop()
}
type baseTimer struct {
	description string
	active      *atomic.Bool
}

func newBaseTimer(description string) *baseTimer {
	return &baseTimer{description: description, active: atomic.NewBool(true)}
}

func (self *baseTimer) Description() string {
	return self.description
}

func (self *baseTimer) Active() bool {
	return self.active.IsSet()
}

func (self *baseTimer) Stop() {
	self.active.UnSet()
}

type onceTimer struct {
	*baseTimer
	t *time.Timer
}

func (self *onceTimer) Stop() {
	if self.active.SetToIf(true, false) {
		self.t.Stop()
		self.t = nil
		log.Debugf("once timer [%s] stop", self.description)
	}
}

func RunAfter(d time.Duration, description string, action func()) TimerId {
	timerId := new(onceTimer)
	timerId.baseTimer = newBaseTimer(description)
	timerId.t = time.AfterFunc(d, func() {
		if timerId.active.SetToIf(true, false) {
			action()
		}
	})
	return timerId
}

type periodTimer struct {
	*baseTimer
	tk        *time.Ticker
	closeChan chan struct{}
}

func (self *periodTimer) Stop() {
	if self.active.SetToIf(true, false) {
		self.tk.Stop()
		close(self.closeChan)
		self.tk = nil
		log.Debugf("period timer [%s] stop", self.description)
	}
}

func RunEvery(interval time.Duration, description string, action func()) TimerId {
	timerId := new(periodTimer)
	timerId.baseTimer = newBaseTimer(description)
	timerId.closeChan = make(chan struct{})
	ticker := time.NewTicker(interval)
	timerId.tk = ticker

	go func() {
		for {
			select {
			case <-ticker.C:
				if timerId.Active() {
					action()
				}
			case <-timerId.closeChan:
				return
			}
		}
	}()
	return timerId
}

type cronTimer struct {
	*baseTimer
	cronExpr       *CronExpr
	customCronExpr *CustomCronExpr
	t              *time.Timer
	tMutex         sync.Mutex
}

func (self *cronTimer) Stop() {
	if self.active.SetToIf(true, false) {
		sync2.With(&self.tMutex, func() {
			if self.t != nil {
				self.t.Stop()
				self.t = nil
			}
		})

		log.Debugf("cron timer [%s] stop", self.description)
	}
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
func Cron(cronExprStr string, description string, action func()) TimerId {
	cronExpr, err := NewCronExpr(cronExprStr)
	if err != nil {
		log.Errorf("description %s parse cronExprStr %s error %s", description, cronExprStr, err)
	}

	timerId := new(cronTimer)
	timerId.baseTimer = newBaseTimer(description)
	timerId.cronExpr = cronExpr

	now := time.Now()
	nextTime := timerId.cronExpr.Next(now)

	if nextTime.IsZero() {
		timerId.active.SetTo(false)
		return timerId
	}
	var cronFunc func()

	cronFunc = func() {
		if !timerId.Active() {
			return
		}

		action()

		now := time.Now()
		nextTime := timerId.cronExpr.Next(now)
		if !nextTime.IsZero() {
			sync2.With(&timerId.tMutex, func() {
				if !timerId.Active() {
					return
				}

				timerId.t = time.AfterFunc(nextTime.Sub(now), cronFunc)
			})
		} else {
			timerId.Stop()
		}
	}

	timerId.t = time.AfterFunc(nextTime.Sub(now), cronFunc)

	return timerId
}

// Customcron 自定义的循环定时任务
// cronExpStr 2018,8,20,22,22,22,5 最后一位为星期;
func Customcron(cronExprStr string, description string, action func()) TimerId {
	cronExpr, err := NewCustomCronExpr(cronExprStr)
	if err != nil {
		log.Errorf("description %s parse cronExprStr %s error %s", description, cronExprStr, err)
	}

	timerId := new(cronTimer)
	timerId.baseTimer = newBaseTimer(description)
	timerId.customCronExpr = cronExpr

	now := time.Now()
	nextTime := timerId.customCronExpr.Next(now)

	if nextTime.IsZero() {
		timerId.active.SetTo(false)
		return timerId
	}
	var cronFunc func()

	cronFunc = func() {
		if !timerId.Active() {
			return
		}

		action()

		now := time.Now()
		nextTime := timerId.customCronExpr.Next(now)
		if !nextTime.IsZero() {
			sync2.With(&timerId.tMutex, func() {
				if !timerId.Active() {
					return
				}

				timerId.t = time.AfterFunc(nextTime.Sub(now), cronFunc)
			})
		} else {
			timerId.Stop()
		}
	}

	timerId.t = time.AfterFunc(nextTime.Sub(now), cronFunc)

	return timerId
}
