package main

import (
	"fmt"
	"github.com/kabu1204/go-pipeline/stream"
	"github.com/kabu1204/go-pipeline/types"
)

type Info struct {
	Age   int
	Intro string
}

type Employee struct {
	Name     string
	Role     string
	Salary   float64
	SelfInfo Info
}

type ComplexStruct struct {
	CField1 string
	CField2 *Employee
	CField3 []string
}

var SimpleTarget = Employee{
	Name:     "ycy",
	Role:     "coder",
	Salary:   400.0,
	SelfInfo: Info{Age: 999},
}

var ComplexTarget = &ComplexStruct{
	CField1: "CValue1",
	CField2: &SimpleTarget,
	CField3: []string{"CValue3", "sadf"},
}

func exampleMapField() {
	a := make(types.Slice, 0, 10)
	for i := 0; i < 10; i++ {
		t := &ComplexStruct{
			CField1: fmt.Sprintf("CValue%d", i),
			CField2: &Employee{
				Name:   fmt.Sprintf("ycy%d", i),
				Role:   fmt.Sprintf("coder", i),
				Salary: 400.0 + float64(i),
				SelfInfo: Info{
					Age:   22 + i,
					Intro: fmt.Sprintf("Hello%d", i),
				},
			},
			CField3: []string{"CValue3", fmt.Sprintf("OK%d", i)},
		}
		a = append(a, t)
	}
	m := stream.Of(a...).MapField("CField2.SelfInfo.Intro").ToSlice()
	fmt.Println(m)
}

func exampleFlatMap() {
	m := stream.Of(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	f := func(i interface{}) interface{} {
		return i.(int) + 2
	}
	m.Map(f).Filter(func(elem interface{}) bool {
		if elem.(int)%2 == 0 {
			return true
		}
		return false
	})
	fmt.Println(m.FlatMap(func(i interface{}) stream.Stream {
		arr := make(types.Slice, 0, 2)
		arr = append(arr, i)
		arr = append(arr, -(i.(int)))
		return stream.Of(arr...)
	}).ToSlice())
}

func exampleSort() {
	m := stream.Of(1, 5, 2, 7, 7, 8, 10, 5, 12, 6, 2, 6, 9, 3, 2, 4, 11)
	cmp := func(a, b interface{}) int {
		return a.(int) - b.(int)
	}
	fmt.Println(m.Sorted(cmp).Limit(10).Skip(3).ToSlice())
}

func exampleReduce() {
	m := stream.Of(1, 5, 2, 7, 7, 8, 10, 5, 12, 6, 2, 6, 9, 3, 2, 4, 11)
	cmp := func(a, b interface{}) int {
		return a.(int) - b.(int)
	}
	result := m.Sorted(cmp).Parallel(20).Distinct(func(i interface{}) int {
		return i.(int) % 100000
	}).Peek(func(e interface{}) {
		fmt.Println("Peek:", e)
	}).Limit(30).Reduce(func(acc, t interface{}) interface{} {
		return acc.(int) + t.(int)
	})
	if !result.IsNone() {
		fmt.Println("Value:", result.Get())
	}
}

func main() {
	exampleMapField()
	exampleFlatMap()
	exampleSort()
	//exampleReduce()
}
