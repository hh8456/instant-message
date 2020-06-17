package csvrecord

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var Comma = '\t'
var Comment = '#'

type Index map[interface{}]interface{}
type CsvRecord struct {
	Comma             rune
	Comment           rune
	typeRecord        reflect.Type
	records           []interface{}
	recordsNum        int
	lock              sync.RWMutex
	indexesMap        map[string]Index
	initLogicCallback func(*CsvRecordTemp)
}

type fieldInfo struct {
	fieldName  string
	fieldType  string
	fieldIndex int
}

const dataOffeset = 4

type Parser interface {
	Parse(string) error
}

// 针对所有配置表加的统一的读写锁
// 一般配置表读取时候用读锁，重载时候加写锁
var allCsvsRWLock = &sync.RWMutex{}

func New(st interface{}, initCallback func(*CsvRecordTemp)) (*CsvRecord, error) {
	typeRecord := reflect.TypeOf(st)

	if typeRecord == nil || typeRecord.Kind() != reflect.Struct {
		return nil, errors.New("st must be a struct")
	}

	for i := 0; i < typeRecord.NumField(); i++ {
		f := typeRecord.Field(i)

		kind := f.Type.Kind()
		switch kind {
		case reflect.String:
		case reflect.Bool:
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		case reflect.Float32, reflect.Float64:
		case reflect.Slice:
		case reflect.Struct:
		case reflect.Ptr:
		default:
			return nil, fmt.Errorf("invalid type: %v %s", f.Name, kind)
		}

		tag := f.Tag.Get("index")
		if tag == "true" {
			switch kind {
			case reflect.Slice:
				return nil, fmt.Errorf("count not index %s field %v %v",
					kind, i, f.Name)
			}
		}
	}

	rf := new(CsvRecord)
	rf.typeRecord = typeRecord
	rf.initLogicCallback = initCallback
	return rf, nil
}

// 重载输入的配置表
// 安全替换，即有一个配置表加载错误，整个加载动作均失败，直接返回error
func ReloadCsvs(csvs []*CsvRecord, fileNames []string) error {
	var tempRecords = make([][]interface{}, 0)
	var tempTableMaps = make([]map[string]Index, 0)
	for i, v := range csvs {
		a, b, e := v.readConfigToTempMemory(fileNames[i])
		if e != nil {
			// 读取配置表报错，回滚
			return fmt.Errorf("reload[%v] error:%v", fileNames[i], e)
		}
		tempRecords = append(tempRecords, a)
		tempTableMaps = append(tempTableMaps, b)
	}
	allCsvsRWLock.Lock()
	for i, v := range csvs {
		// 加锁替换当前csv内容
		v.replaceNewData(tempRecords[i], tempTableMaps[i])
	}
	allCsvsRWLock.Unlock()
	return nil
}

// 读取单个配置表，读取失败返回error
func (cr *CsvRecord) Read(fileName string) error {
	// 读配置内容到临时内存空间
	r, m, e := cr.readConfigToTempMemory(fileName)
	if e != nil {
		return e
	}

	// 加锁替换当前csv内容
	cr.replaceNewData(r, m)

	return nil
}

//返回第i+1条数据 i < NumRecord()
func (cr *CsvRecord) Record(i int) interface{} {
	var ret interface{}

	allCsvsRWLock.RLock()
	cr.lock.RLock()
	if 0 <= i && i < cr.recordsNum {
		ret = cr.records[i]
	}
	cr.lock.RUnlock()
	allCsvsRWLock.RUnlock()

	return ret
}

//返回有多少条数据
func (cr *CsvRecord) NumRecrod() int {
	var ret int
	allCsvsRWLock.RLock()
	cr.lock.RLock()
	ret = cr.recordsNum
	cr.lock.RUnlock()
	allCsvsRWLock.RUnlock()
	return ret
}

//不存在返回 nil
// fieldName 为csv表中 与 struct tag 中标注为 “index:true" 对应的名字
func (cr *CsvRecord) Index(fieldName string, key interface{}) interface{} {
	var ret interface{}
	allCsvsRWLock.RLock()
	cr.lock.RLock()
	index, ok := cr.indexesMap[fieldName]
	if ok {
		ret = index[key]
	}
	cr.lock.RUnlock()
	allCsvsRWLock.RUnlock()
	return ret
}

// 读取配置表内容到一个临时内存空间
func (cr *CsvRecord) readConfigToTempMemory(fileName string) ([]interface{},
	map[string]Index, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	if cr.Comma == 0 {
		cr.Comma = Comma
	}
	if cr.Comment == 0 {
		cr.Comment = Comment
	}

	reader := csv.NewReader(file)
	reader.Comma = cr.Comma
	reader.Comment = cr.Comment
	fieldTypeLine, err := reader.Read()
	if err != nil {
		return nil, nil, err
	}

	fieldNameLine, err := reader.Read()
	if err != nil {
		return nil, nil, err
	}

	dataLines, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}

	typeRecord := cr.typeRecord
	fieldInfoMap := make(map[string]*fieldInfo)
	for i := 0; i < len(fieldNameLine); i++ {
		fieldInfo := &fieldInfo{
			fieldName:  fieldNameLine[i],
			fieldType:  fieldTypeLine[i],
			fieldIndex: i,
		}
		fieldInfoMap[fieldInfo.fieldName] = fieldInfo
	}

	indexesMap := make(map[string]Index)
	for i := 0; i < typeRecord.NumField(); i++ {
		tag := typeRecord.Field(i).Tag.Get("index")
		if tag == "true" {
			indexesMap[typeRecord.Field(i).Tag.Get("csv")] = make(Index)
		}
	}

	records := make([]interface{}, len(dataLines))
	for n := 0; n < len(dataLines); n++ {
		value := reflect.New(typeRecord)
		records[n] = value.Interface()
		record := value.Elem()

		dataLine := dataLines[n]

		for i := 0; i < typeRecord.NumField(); i++ {
			f := typeRecord.Field(i)
			fieldTagName := f.Tag.Get("csv")
			fInfo := fieldInfoMap[fieldTagName]
			if fInfo == nil {
				return nil, nil, fmt.Errorf("line[%v] tag [%s] not exist filedinfo", n, f.Tag)
			}
			fieldStr := dataLine[fInfo.fieldIndex]

			field := record.Field(i)

			if !field.CanSet() {
				continue
			}

			var err error

			err = setValue(field, fieldStr)

			if err != nil {
				return nil, nil,
					fmt.Errorf("parse field (row=%v, col=%s, type=%s) error: %v", n+dataOffeset, fInfo.fieldName, fInfo.fieldType, err)
			}
			if f.Tag.Get("index") == "true" {
				index := indexesMap[f.Tag.Get("csv")]
				if _, ok := index[field.Interface()]; ok {
					return nil, nil,
						fmt.Errorf("duplicate index %v at %v (row=%v, col=%v)", field.Interface(), fileName, n+dataOffeset, i)
				}
				index[field.Interface()] = records[n]
			}
		}
	}
	return records, indexesMap, nil
}

// 加锁替换csv结构的内容
func (cr *CsvRecord) replaceNewData(r []interface{}, m map[string]Index) {
	cr.lock.Lock()
	cr.records = r
	cr.recordsNum = len(r)
	cr.indexesMap = m

	// 拷贝一个临时变量，用作配置加载时，针对
	// 配置表额外做的工作回调参数
	tempCr := new(CsvRecordTemp)
	if cr.initLogicCallback != nil {
		tempCr.records = append(tempCr.records, r...)
		tempCr.recordsNum = len(r)
		tempCr.indexesMap = make(map[string]Index)
		for k, v := range m {
			tempCr.indexesMap[k] = v
		}
	}

	cr.lock.Unlock()

	// 调用回调函数，初始化额外结构
	if cr.initLogicCallback != nil {
		cr.initLogicCallback(tempCr)
	}
}

//变量赋值
func setValue(field reflect.Value, value string) error {
	if field.Kind() == reflect.Ptr {
		if value == "" {
			return nil
		}

		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value == "" {
			field.SetInt(0)
		} else {
			i, err := strconv.ParseInt(value, 0, field.Type().Bits())
			if err != nil {
				return err
			}
			field.SetInt(i)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ui, err := strconv.ParseUint(value, 0, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(ui)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(f)
	case reflect.Struct:
		field = field.Addr()
		parser, ok := field.Interface().(Parser)
		if !ok {
			return fmt.Errorf("struct no implement Parser")
		}
		err := parser.Parse(value)
		if err != nil {
			fmt.Errorf("%s parse error %s", field.Type(), err)
		}
	case reflect.Slice:
		values := strings.Split(value, ",")
		if len(values) == 1 && values[0] == "" {
			values = []string{}
		}
		field.Set(reflect.MakeSlice(field.Type(), len(values), len(values)))
		for i := 0; i < len(values); i++ {
			err := setValue(field.Index(i), values[i])
			if err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("no support type %s", field.Type())
	}
	return nil
}
