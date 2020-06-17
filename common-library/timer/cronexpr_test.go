package timer

import (
	"testing"
	"time"
)

func test(expr string, t *testing.T) {

	cronExpr, _ := NewCronExpr(expr)
	t.Log("expr: ", expr)
	current := time.Now()
	for i := 0; i < 5; i++ {
		t.Log("current: ", current)
		t.Log("next: ", cronExpr.Next(current))
		current = cronExpr.Next(current)
	}
}

func TestCronExpr(t *testing.T) {
	test("* * * * * *", t)
	test("0 0 0 * * *", t)
	test("0 0 0,1,2 * * *", t)
	test("0 0 0 * * 1", t)
}
