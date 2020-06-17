package csvrecord

import (
	"reflect"
	"sync"
)

type CsvRecordTemp struct {
	Comma             rune
	Comment           rune
	typeRecord        reflect.Type
	records           []interface{}
	recordsNum        int
	lock              sync.RWMutex
	indexesMap        map[string]Index
	initLogicCallback func(*CsvRecord)
}

func (crt *CsvRecordTemp) Record(i int) interface{} {
	var ret interface{}

	if 0 <= i && i < crt.recordsNum {
		ret = crt.records[i]
	}

	return ret
}

func (crt *CsvRecordTemp) NumRecrod() int {
	var ret int
	ret = crt.recordsNum
	return ret
}

func (crt *CsvRecordTemp) Index(fieldName string, key interface{}) interface{} {
	var ret interface{}
	index, ok := crt.indexesMap[fieldName]
	if ok {
		ret = index[key]
	}
	return ret
}
