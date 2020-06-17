package eventmanager

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
)

// concurrent test subscribe
func testEventHandlerConcurrentSub(t *testing.T) {
	// go test -> 3 goroutine, cost 7.8s for 1M times
	println("concurrent test subscribe")
	for j := 0; j < 1000000; j++ {
		r := make(chan int, 3)
		instance = &eventManager{
			handlers: &sync.Map{},
		}
		go func(r1 chan int) {
			instance.Subscribe("system1", EventID(1), handlercb1)
			instance.Subscribe("system1", EventID(2), handlercb2)
			instance.Subscribe("system1", EventID(3), handlercb3)
			r1 <- 1
		}(r)
		go func(r1 chan int) {
			instance.Subscribe("system2", EventID(1), handlercb1)
			instance.Subscribe("system2", EventID(2), handlercb2)
			instance.Subscribe("system2", EventID(3), handlercb3)
			r1 <- 1
		}(r)
		go func(r1 chan int) {
			instance.Subscribe("system3", EventID(1), handlercb1)
			instance.Subscribe("system3", EventID(2), handlercb2)
			instance.Subscribe("system3", EventID(3), handlercb3)
			r1 <- 1
		}(r)
		for i := 0; i < 3; i++ {
			<-r
		}

		f := func(no int) {
			if v, ok := instance.handlers.Load(EventID(no)); ok {
				eventIDMap := v.(*sync.Map) // subscriberName - eventHandler
				if _, ok = eventIDMap.Load("system" + strconv.Itoa(no)); ok {

				} else {
					t.Fatalf("[%v]eventid %v not found system%d", j, no, no)
				}
			} else {
				t.Fatalf("[%v]eventid %v not found", j, no)
			}
		}
		for i := 1; i < 4; i++ {
			f(i)
		}
	}

	println("concurrent test subscribe")
}

func TestEventHandlerConcurrentSubPub(t *testing.T) {
	// go test -> 3 subscribers, 10 publishers concurrent publish
	// 			  1000000 times cost 35s, 35us/op (note, the cost
	//			  time consist of 1M times' initialtion/creating
	// 			  3 subscribers)
	println("concurrent test subscribe publish")

	var wg sync.WaitGroup
	wg.Add(8000)
	for j := 0; j < 8000; j++ {
		go func() {
			Subscribe("system1", EventID(1), handlercb1)
			Subscribe("system1", EventID(2), handlercb2)
			Subscribe("system1", EventID(3), handlercb3)
			wg.Done()
		}()
	}

	wg.Wait()

	var wg1 sync.WaitGroup

	wg1.Add(8000)
	for j := 0; j < 8000; j++ {
		tmp := j
		go func() {
			evntid := 1
			eventArg := NewEventArg()
			eventArg.SetInt32("index", int32(tmp))
			eventArg.SetInt32("testint32", int32(evntid))
			Publish(EventID(evntid), eventArg)

			evntid = 2
			eventArg = NewEventArg()
			eventArg.SetInt32("index", int32(tmp))
			eventArg.SetInt32("testint32", int32(evntid))
			Instance().Publish(EventID(evntid), eventArg)

			evntid = 3
			eventArg = NewEventArg()
			eventArg.SetInt32("index", int32(tmp))
			eventArg.SetInt32("testint32", int32(evntid))
			Instance().Publish(EventID(evntid), eventArg)
			//r1 <- 1
			wg1.Done()
		}()
	}

	wg1.Wait()

	println("concurrent test subscribe publish over")
}

var l1 sync.Mutex
var l2 sync.Mutex
var l3 sync.Mutex
var count1 = 0
var count2 = 0
var count3 = 0

// concurrent test publish
func testEventHandlerConcurrentPub(t *testing.T) {
	// go test -> 3 subscribers, 10 publishers concurrent publish
	// 			  1000000 times cost 35s, 35us/op (note, the cost
	//			  time consist of 1M times' initialtion/creating
	// 			  3 subscribers)
	println("concurrent test publish")
	for j := 0; j < 1000000; j++ {
		r := make(chan int)
		instance = &eventManager{
			handlers: &sync.Map{},
		}
		go func(r1 chan int) {
			Instance().Subscribe("system1", EventID(1), handlercb1)
			Instance().Subscribe("system1", EventID(2), handlercb2)
			Instance().Subscribe("system1", EventID(3), handlercb3)
			r1 <- 1
		}(r)
		go func(r1 chan int) {
			Instance().Subscribe("system2", EventID(1), handlercb1)
			Instance().Subscribe("system2", EventID(2), handlercb2)
			Instance().Subscribe("system2", EventID(3), handlercb3)
			r1 <- 1
		}(r)
		go func(r1 chan int) {
			Instance().Subscribe("system3", EventID(1), handlercb1)
			Instance().Subscribe("system3", EventID(2), handlercb2)
			Instance().Subscribe("system3", EventID(3), handlercb3)
			r1 <- 1
		}(r)
		for i := 0; i < 3; i++ {
			<-r
		}

		pubCount := 10
		for i := 1; i <= pubCount; i++ {
			tmp := j
			go func(r1 chan int) {
				evntid := 1
				eventArg := NewEventArg()
				eventArg.SetInt32("index", int32(tmp))
				eventArg.SetInt32("testint32", int32(evntid))
				Instance().Publish(EventID(evntid), eventArg)

				evntid = 2
				eventArg = NewEventArg()
				eventArg.SetInt32("index", int32(tmp))
				eventArg.SetInt32("testint32", int32(evntid))
				Instance().Publish(EventID(evntid), eventArg)

				evntid = 3
				eventArg = NewEventArg()
				eventArg.SetInt32("index", int32(tmp))
				eventArg.SetInt32("testint32", int32(evntid))
				Instance().Publish(EventID(evntid), eventArg)
				r1 <- 1
			}(r)
		}

		for i := 0; i < pubCount; i++ {
			<-r
		}
	}

	if count1 != 30000000 {
		t.Fatalf("count1(%v)", count1)
	}
	if count2 != 30000000 {
		t.Fatalf("count2(%v)", count2)
	}
	if count3 != 30000000 {
		t.Fatalf("count3(%v)", count3)
	}

	println("concurrent test publish over")
}

func testEventHandlerUnsub(t *testing.T) {
	instance = &eventManager{
		handlers: &sync.Map{},
	}
	instance.Subscribe("system1", EventID(1), handlercb1)
	instance.Subscribe("system1", EventID(2), handlercb2)
	if v, ok := instance.handlers.Load(EventID(1)); ok {
		//f := v.(eventMap)["system1"]
		v2, ok2 := v.(*sync.Map).Load("system1")
		if ok2 {
			evntid := 1
			eventArg := NewEventArg()
			eventArg.SetInt32("index", int32(1))
			eventArg.SetInt32("testint32", int32(evntid))
			f := v2.(EventHandler)
			f(eventArg)
		} else {
			t.Fatalf("system1")
		}
	} else {
		t.Fatalf("not found eventid 1")
	}
	instance.UnSubscribe("system1", EventID(1))
	if _, ok := instance.handlers.Load(EventID(1)); !ok {

	} else {
		t.Fatalf("after unsub, but found eventid 1")
	}
	if _, ok := instance.handlers.Load(EventID(2)); ok {

	} else {
		t.Fatalf("not found eventid 2")
	}
}

func handlercb1(event *EventArg) {
	l1.Lock()
	count1++
	l1.Unlock()

	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(1) {
		panic(fmt.Errorf("[%v]cb1 arg not 1", index))
	}
}

func handlercb2(event *EventArg) {
	l1.Lock()
	count2++
	l1.Unlock()

	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(2) {
		panic(fmt.Errorf("[%v]cb1 arg not 2", index))
	}
}

func handlercb3(event *EventArg) {
	l1.Lock()
	count3++
	l1.Unlock()

	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(3) {
		panic(fmt.Errorf("[%v]cb1 arg not 3", index))
	}
}

func handlercb4(event *EventArg) {
	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(4) {
		panic(fmt.Errorf("[%v]cb1 arg not 4", index))
	}
}

func handlercb5(event *EventArg) {
	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(5) {
		panic(fmt.Errorf("[%v]cb1 arg not 5", index))
	}
}

func handlercb6(event *EventArg) {
	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(6) {
		panic(fmt.Errorf("[%v]cb1 arg not 6", index))
	}
}

func handlercb7(event *EventArg) {
	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(7) {
		panic(fmt.Errorf("[%v]cb1 arg not 7", index))
	}
}

func handlercb8(event *EventArg) {
	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(8) {
		panic(fmt.Errorf("[%v]cb1 arg not 8", index))
	}
}

func handlercb9(event *EventArg) {
	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(9) {
		panic(fmt.Errorf("[%v]cb1 arg not 9", index))
	}
}

func handlercb10(event *EventArg) {
	argint32 := event.GetInt32("testint32")
	index := event.GetInt32("index")
	if argint32 != int32(10) {
		panic(fmt.Errorf("[%v]cb1 arg not 10", index))
	}
}
