package stream

import (
	"github.com/kabu1204/go-pipeline/types"
	"reflect"
)

func Slice(elems interface{}) []interface{} {
	if reflect.TypeOf(elems).Kind() != reflect.Slice {
		return nil
	}
	valueOfElems := reflect.ValueOf(elems)
	n := valueOfElems.Len()
	slice := make(types.Slice, 0, n)
	for i := 0; i < n; i++ {
		slice = append(slice, valueOfElems.Index(i).Interface())
	}
	return slice
}

func Of(elems ...interface{}) *stream {
	slice := types.Slice(elems)
	return &stream{
		source:  (&slice).Iterator(),
		prev:    nil,
		wrapper: defaultWrapper,
		Name:    "Of",
	}
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}
