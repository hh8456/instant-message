package csvrecord

import (
	"fmt"
	"strings"
	"testing"
)

type Person struct {
	FirstName  string
	SecondName string
}

func (self *Person) Parse(text string) error {
	strs := strings.Split(text, ":")
	self.FirstName = strs[0]
	self.SecondName = strs[1]
	return nil
}

//tag 标签中的名字与csv表中相同 该字段需要作为索引字段时请在 tag 中添加 index:trues
type Address struct {
	Id     int     `csv:"id" index:"true"`
	Name   string  `csv:"name" index:"true"`
	Email  string  `csv:"email"`
	Height float32 `csv:"height"`
	Keys   []int   `csv:"keys"`
	IsHome bool    `csv:"ishome"`

	I8         int8      `csv:"i8"`
	I16        int16     `csv:"i16"`
	I32        int32     `csv:"i32"`
	I64        int64     `csv:"i64"`
	UInt       uint      `csv:"uint"`
	UI8        uint8     `csv:"ui8"`
	UI16       uint16    `csv:"ui16"`
	UI32       uint32    `csv:"ui32"`
	UI64       uint64    `csv:"ui64"`
	Float32    float32   `csv:"float32"`
	Float64    float64   `csv:"float64"`
	Person     Person    `csv:"person"`
	PtrPerson  *Person   `csv:"ptrperson"`
	Persons    []Person  `csv:"persons"`
	PtrPersons []*Person `csv:"persons"`
}

func (self *Address) ToString() string {
	return fmt.Sprintf("[%d] [%s] [%s] [%f]", self.Id, self.Name, self.Email, self.Height)
}

func TestRead(t *testing.T) {
	cr, err := New(Address{})
	if err != nil {
		t.Fatal("New error ", err)
	}

	err = cr.Read("test.csv")
	if err != nil {
		t.Fatal("Read error ", err)
	}

	if len(cr.indexesMap) != 2 {
		t.Fatal("indexesMap size != 2")
	}

	if len(cr.records) != 3 {
		t.Fatal("records != 3")
	}

	for i := 0; i < cr.NumRecrod(); i++ {
		t.Log(cr.Record(i))
	}

	for i := 1001; i <= 1004; i++ {
		t.Log(i, " : ", cr.Index("id", i))
	}

	t.Log("John Done : ", cr.Index("name", "John Done"))
	t.Log("test1 : ", cr.Index("name", "test1"))
	t.Log("test2 : ", cr.Index("name", "test2"))
	t.Log("test3 : ", cr.Index("name", "test3"))

	test1 := cr.Index("name", "test1").(*Address)
	t.Log("PtrPerson ", *test1.PtrPerson)
	t.Log("PtrPersons:")
	for _, v := range test1.PtrPersons {
		t.Log(*v)
	}
}

func TestIndex(t *testing.T) {
	cr, _ := New(Address{})
	cr.Read("test.csv")

	for i := 1001; i <= 1003; i++ {
		adderss := cr.Index("id", i).(*Address)
		if adderss.Id != i {
			t.Fatal(i, ": ", "adderss.Id != i")
		}
	}
	adderss := cr.Index("id", 1004)
	if adderss != nil {
		t.Fatal(1004, "adderss != nil")
	}

	names := []string{"John Done", "test1", "test2"}

	for _, v := range names {
		adderss := cr.Index("name", v).(*Address)
		if adderss.Name != v {
			t.Fatal(v, " : ", adderss.Name != v)
		}
	}

	adderss = cr.Index("name", "test4")
	if adderss != nil {
		t.Fatal(1004, "adderss != nil")
	}
}
