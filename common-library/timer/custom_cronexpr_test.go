package timer

import (
	"fmt"
	"testing"
	"time"
)

func testCustomCronExp(expr string, t *testing.T) {

	cronExpr, _ := NewCustomCronExpr(expr)
	t.Log("expr: ", expr)
	fmt.Printf("expr: %v\n", expr)
	// fmt.Printf("cronExpr: %+v\n", cronExpr)
	current := time.Now()
	// current := time.Now().AddDate(0, -5, 0)
	// for i := 0; i < 5; i++ {
	// 	t.Log("curr: ", current)
	// 	fmt.Printf("curr: %v\n", current)
	// 	t.Log("next: ", cronExpr.Next(current))
	// 	fmt.Printf("next: %v\n", cronExpr.Next(current))
	// 	current = cronExpr.Next(current)
	// }
	for i := 0; i < 5; i++ {
		t.Log("curr: ", current)
		fmt.Printf("curr: %v\n", current)
		t.Log("prev: ", cronExpr.Prev(current))
		fmt.Printf("prev: %v\n", cronExpr.Prev(current))
		current = cronExpr.Prev(current)
	}
}

func TestCustomCronExpr(t *testing.T) {
	// testCustomCronExp("2018,9,22,18,2,*", t)
	// testCustomCronExp("2018,9,22,18,*,0", t)
	// testCustomCronExp("2018,9,22,*,0,0", t)
	// testCustomCronExp("2018,9,*,12,0,0", t)
	// testCustomCronExp("2018,*,8,12,0,0", t)
	// testCustomCronExp("*,9,8,12,0,0", t)
	// testCustomCronExp("2018,9,8,12,0,0,2", t)
	// testCustomCronExp("2018,9,*,*,0,0,2", t)
	// testCustomCronExp("2018,*,*,12,0,0,2", t)
	// testCustomCronExp("2018,9,*,12,0,0,0", t)
	testCustomCronExp("*,*,*,18,12,0", t)
}
