package eventmanager

import (
	"bytes"
	"fmt"
)

type EventArg struct {
	int64ArgMap  map[string]int64
	int32ArgMap  map[string]int32
	boolArgMap   map[string]bool
	stringArgMap map[string]string
	userDataMap  map[string]interface{}
}

func NewEventArg() *EventArg {
	return &EventArg{
		int64ArgMap:  make(map[string]int64),
		int32ArgMap:  make(map[string]int32),
		boolArgMap:   make(map[string]bool),
		stringArgMap: make(map[string]string),
		userDataMap:  make(map[string]interface{}),
	}
}

func (self *EventArg) String() string {
	buffer := bytes.NewBuffer(nil)
	fmt.Fprintf(buffer, "{")

	for k, v := range self.int64ArgMap {
		fmt.Fprintf(buffer, " {%s:%d} ", k, v)
	}

	for k, v := range self.int32ArgMap {
		fmt.Fprintf(buffer, " {%s:%d} ", k, v)
	}

	for k, v := range self.boolArgMap {
		fmt.Fprintf(buffer, " {%s:%t} ", k, v)
	}

	for k, v := range self.stringArgMap {
		fmt.Fprintf(buffer, " {%s:%s} ", k, v)
	}

	for k, v := range self.userDataMap {
		fmt.Fprintf(buffer, " {%s:%v} ", k, v)
	}

	fmt.Fprintf(buffer, "}")

	return buffer.String()
}
func (self *EventArg) SetInt64(k string, v int64) {
	self.int64ArgMap[k] = v
}

func (self *EventArg) GetInt64(k string) int64 {
	return self.int64ArgMap[k]
}

func (self *EventArg) SetInt32(k string, v int32) {
	self.int32ArgMap[k] = v
}

func (self *EventArg) GetInt32(k string) int32 {
	return self.int32ArgMap[k]
}
func (self *EventArg) SetBool(k string, v bool) {
	self.boolArgMap[k] = v
}

func (self *EventArg) GetBool(k string) bool {
	return self.boolArgMap[k]
}

func (self *EventArg) SetString(k string, v string) {
	self.stringArgMap[k] = v
}

func (self *EventArg) GetString(k string) string {
	return self.stringArgMap[k]
}

func (self *EventArg) SetUserData(k string, v interface{}) {
	self.userDataMap[k] = v
}

func (self *EventArg) GetUserData(k string) interface{} {
	return self.userDataMap[k]
}
