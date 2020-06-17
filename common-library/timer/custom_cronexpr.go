package timer

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// CustomCronExpr 自定义定时解释结构
type CustomCronExpr struct {
	sec   uint64
	min   uint64
	hour  uint64
	dom   uint64
	month uint64
	dow   uint64
	year  int
}

// NewCustomCronExpr 创建新的自定义定时解释结构
// expr 2018,8,20,22,22,22,5 最后一位为星期;若日期与星期同时存在，则同时生效
func NewCustomCronExpr(expr string) (ccronExprPtr *CustomCronExpr, err error) {
	splitFunc := func(c rune) bool {
		return c == ','
	}
	fieldArr := strings.FieldsFunc(expr, splitFunc)
	if len(fieldArr) != 6 && len(fieldArr) != 7 {
		err = fmt.Errorf("invalid expr %v: expected 6 or 7 fields, got %v", expr, len(fieldArr))
		return
	}
	if len(fieldArr) == 6 {
		fieldArr = append(fieldArr, "*")
	}

	ccronExprPtr = new(CustomCronExpr)
	// Year
	if fieldArr[0] == "*" {
		ccronExprPtr.year = math.MaxInt32
	} else {
		ccronExprPtr.year, err = strconv.Atoi(fieldArr[0])
		if err != nil {
			err = fmt.Errorf("year covert err: %v", err)
			return
		}
	}
	// Month
	ccronExprPtr.month, err = parseCustomCronField(fieldArr[1], 1, 12)
	if err != nil {
		err = fmt.Errorf("invalid expr %v: %v", expr, err)
		return
	}
	// Day of month
	ccronExprPtr.dom, err = parseCustomCronField(fieldArr[2], 1, 31)
	if err != nil {
		err = fmt.Errorf("invalid expr %v: %v", expr, err)
		return
	}
	// Hours
	ccronExprPtr.hour, err = parseCustomCronField(fieldArr[3], 0, 23)
	if err != nil {
		err = fmt.Errorf("invalid expr %v: %v", expr, err)
		return
	}
	// Minutes
	ccronExprPtr.min, err = parseCustomCronField(fieldArr[4], 0, 59)
	if err != nil {
		err = fmt.Errorf("invalid expr %v: %v", expr, err)
		return
	}
	// Seconds
	ccronExprPtr.sec, err = parseCustomCronField(fieldArr[5], 0, 59)
	if err != nil {
		err = fmt.Errorf("invalid expr %v: %v", expr, err)
		return
	}
	// Day of week
	ccronExprPtr.dow, err = parseCustomCronField(fieldArr[6], 0, 6)
	if err != nil {
		err = fmt.Errorf("invalid expr %v: %v", expr, err)
		return
	}

	return
}

// parseCustomCronField
// 1.*
// 2.num
func parseCustomCronField(field string, min int, max int) (cronField uint64, err error) {
	var start, end int

	if field == "*" {
		start = min
		end = max
	} else { // 常数
		start, err = strconv.Atoi(field)
		if err != nil {
			return
		}
		end = start
	}

	// cronField
	// fmt.Printf("start: %v, end: %v\n", start, end)
	cronField |= ^(math.MaxUint64 << uint(end+1)) & (math.MaxUint64 << uint(start))
	// fmt.Printf("cronField: %64b\n", cronField)

	return
}

func (e *CustomCronExpr) matchDay(t time.Time) bool {
	// day-of-month blank
	if e.dom == 0xfffffffe {
		return 1<<uint(t.Weekday())&e.dow != 0
	}

	// day-of-week blank
	if e.dow == 0x7f {
		return 1<<uint(t.Day())&e.dom != 0
	}

	return 1<<uint(t.Weekday())&e.dow != 0 ||
		1<<uint(t.Day())&e.dom != 0
}

// Next get next run time, goroutine safe
func (e *CustomCronExpr) Next(t time.Time) time.Time {
	// the upcoming second
	t = t.Truncate(time.Second).Add(time.Second)

	initFlag := false

retry:
	// Year
	if t.Year() != e.year && e.year != math.MaxInt32 {
		return time.Time{}
	}

	// Month
	for 1<<uint(t.Month())&e.month == 0 {
		if !initFlag { // 当前月不会触发init，即导致计算出day在当前日期之前的情况
			initFlag = true
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		}

		t = t.AddDate(0, 1, 0)
		if t.Month() == time.January {
			goto retry
		}
	}

	// Day
	for !e.matchDay(t) {
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		}

		t = t.AddDate(0, 0, 1)
		if t.Day() == 1 {
			goto retry
		}
	}

	// Hours
	for 1<<uint(t.Hour())&e.hour == 0 {
		if !initFlag {
			initFlag = true
			t = t.Truncate(time.Hour)
		}

		t = t.Add(time.Hour)
		if t.Hour() == 0 {
			goto retry
		}
	}

	// Minutes
	for 1<<uint(t.Minute())&e.min == 0 {
		if !initFlag {
			initFlag = true
			t = t.Truncate(time.Minute)
		}

		t = t.Add(time.Minute)
		if t.Minute() == 0 {
			goto retry
		}
	}

	// Seconds
	for 1<<uint(t.Second())&e.sec == 0 {
		if !initFlag {
			initFlag = true
		}

		t = t.Add(time.Second)
		if t.Second() == 0 {
			goto retry
		}
	}

	return t
}

// Prev get prev run time, goroutine safe
func (e *CustomCronExpr) Prev(t time.Time) time.Time {
	// the past second 上一秒
	t = t.Truncate(time.Second).Add(-time.Second)

	initFlag := false

retry:
	// Year
	if t.Year() != e.year && e.year != math.MaxInt32 {
		return time.Time{}
	}

	// Month
	for 1<<uint(t.Month())&e.month == 0 {
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), 0, 23, 59, 59, 0, t.Location())
			t = t.AddDate(0, 1, 0)
		}

		d := t.Day()
		t = t.AddDate(0, -1, 0)
		if d != t.Day() { // 每月最后一天可能会出现
			t = time.Date(t.Year(), t.Month(), 0, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
			t = t.AddDate(0, 1, 0)
		}
		if t.Month() == time.December {
			goto retry
		}
	}

	// Day
	for !e.matchDay(t) {
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
		}

		curMonth := t.Month()
		t = t.AddDate(0, 0, -1)
		prevMonth := t.Month()
		if curMonth != prevMonth {
			goto retry
		}
	}

	// Hours
	for 1<<uint(t.Hour())&e.hour == 0 {
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 59, 59, 0, t.Location())
		}

		t = t.Add(-time.Hour)
		if t.Hour() == 23 {
			goto retry
		}
	}

	// Minutes
	for 1<<uint(t.Minute())&e.min == 0 {
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 59, 0, t.Location())
		}

		t = t.Add(-time.Minute)
		if t.Minute() == 59 {
			goto retry
		}
	}

	// Seconds
	for 1<<uint(t.Second())&e.sec == 0 {
		if !initFlag {
			initFlag = true
		}

		t = t.Add(-time.Second)
		if t.Second() == 59 {
			goto retry
		}
	}

	return t
}
