package stimer

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

// 并发测试
func TestGetInstance(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1000)

	for i := 0; i < 1000; i++ {

		go func() {
			Instance()

			wg.Done()
		}()
	}

	wg.Wait()

	println("TestGetInstance over")
}

func TestRunEver(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1000)

	var mapTimerID sync.Map
	for i := 0; i < 1000; i++ {

		go func(m int) {
			description := "描述 " + strconv.Itoa(m)
			id := Instance().RunEver(time.Second, description, func() {
				// do something
				for n := 0; n < 3000; n++ {
					k := m % 2
					if k == 0 {
						k++
					}
				}
			})
			mapTimerID.Store(id, 0)
			wg.Done()
		}(i)
	}

	wg.Wait()

	mapTimerID.Range(func(key, value interface{}) bool {
		Instance().StopTimer(key.(int64))
		return true
	})

	println("TestRunEver over")

}

func TestRunAfter(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1000)

	for i := 0; i < 1000; i++ {

		go func(m int) {
			description := "描述 " + strconv.Itoa(m)
			Instance().RunAfter(time.Second, description, func() {
				// do something
				for n := 0; n < 3000; n++ {
					k := m % 2
					if k == 0 {
						k++
					}
				}
			})
			wg.Done()
		}(i)
	}

	wg.Wait()

	println("TestRunAfter over")

}
