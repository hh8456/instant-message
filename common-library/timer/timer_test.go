package timer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTimer(t *testing.T) {
	var wait sync.WaitGroup

	start := time.Now()

	wait.Add(1)
	RunAfter(time.Second, "one seconde", func() {
		t.Log("one seconde ", time.Now().Sub(start).Seconds())
		wait.Done()
	})

	wait.Add(1)
	RunAfter(time.Second*2, "two seconde", func() {
		t.Log("two seconde ", time.Now().Sub(start).Seconds())
		wait.Done()
	})

	wait.Add(1)
	timerId := RunAfter(time.Second*3, "Will not print", func() {
		t.Log("will not print")
		wait.Done()
	})
	timerId.Stop()
	wait.Done()

	timerId = RunEvery(time.Second, "every seconde", func() {
		t.Log("every second ", time.Now().Sub(start).Seconds())
	})

	wait.Add(1)
	RunAfter(time.Second*10, "stop every second", func() {
		t.Log("stop every second ", time.Now().Sub(start).Seconds())
		timerId.Stop()
		wait.Done()
	})
	wait.Wait()
}

func TestCustomCronTimer(t *testing.T) {
	var waite sync.WaitGroup

	waite.Add(1)
	Customcron("2018,8,24,*,*,*", "nothing", func() {
		fmt.Println("executed func")
		waite.Done()
	})
	waite.Wait()
}
