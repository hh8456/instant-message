package csvrecord

import "fmt"

var allCsvConfigs []*CsvRecord
var allCsvFilePaths []string

func init() {
	allCsvConfigs = make([]*CsvRecord, 0)
	allCsvFilePaths = make([]string, 0)
}

// 注册一个配置表，initLogicFun是在加载/重载配置表时，给
// 一个临时拷贝的配置表内容，用于额外的数据生成
func RegisterConfig(table interface{}, path string,
	initLogicFun func(*CsvRecordTemp)) (*CsvRecord, error) {
	csv, err := New(table, initLogicFun)
	if err != nil {
		return nil, fmt.Errorf("register config[%v] error:%v", path, err)
	}
	allCsvConfigs = append(allCsvConfigs, csv)
	allCsvFilePaths = append(allCsvFilePaths, path)
	return csv, nil
}

// 加载所有注册的配置表，读取某个表失败即返回错误，但不支持事务加载
func LoadAll() error {
	for i, v := range allCsvConfigs {
		err := v.Read(allCsvFilePaths[i])
		if err != nil {
			return fmt.Errorf("load[%v] error:%v", allCsvFilePaths[i], err)
		}
	}
	return nil
}

// 加载所有注册的配置表，读取某个表失败即返回错误，
// 事务加载！读取某个表失败，则所有表加载动作失效，不替换任何表数据！
func ReloadAll() error {
	err := ReloadCsvs(allCsvConfigs, allCsvFilePaths)
	if err != nil {
		// 更新报错了，不替换
		return err
	}
	return nil
}
