package main

import (
	"code.byted.org/yuchengye/pipeline/stream"
	"fmt"
)

func exampleLazyFilter() {
	m := stream.Of(1, 2, 3, 4, 5, 6).Filter(func(i interface{}) bool {
		return i.(int)%2 == 0
	}).ToSlice()
	fmt.Println(m)
}

func main() {
	exampleLazyFilter()
}
