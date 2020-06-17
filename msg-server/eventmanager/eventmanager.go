package eventmanager

import (
	"instant-message/common-library/function"
	"sync"

	"fmt"
)

type eventManager struct {
	handlers *sync.Map
}

var instance *eventManager

func init() {
	instance = &eventManager{
		handlers: &sync.Map{}, // eventID - *sync.Map
	}
}

func Instance() *eventManager {
	return instance
}

func Subscribe(subscriberName string, eventId EventID,
	eventHandler EventHandler) {
	newv := &sync.Map{}
	newv.Store(subscriberName, eventHandler)
	v, loaded := instance.handlers.LoadOrStore(eventId, newv)
	if loaded {
		_, loaded2 := v.(*sync.Map).LoadOrStore(subscriberName, eventHandler)
		if loaded2 {
			//log.Errorf("game, 重复注册事件, subscriberName: %s, eventId: %d", subscriberName, int(eventId))
		}
	}

}

func UnSubscribe(subscriberName string, eventID EventID) {
	instance.handlers.Delete(eventID)
}

func Publish(eventID EventID, eventArg *EventArg) {
	v, ok := instance.handlers.Load(eventID)
	if !ok {
		return
	}

	eventIDMap := v.(*sync.Map) // subscriberName - eventHandler
	eventIDMap.Range(func(key, value interface{}) bool {
		defer function.CatchWithInfo(fmt.Sprintf("eventid:%v, system:%v panic!", eventID, key))
		value.(EventHandler)(eventArg)

		return true
	})
}
