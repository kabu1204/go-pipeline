package types

import (
	"reflect"
	"strings"
)

func GetFieldInterfaceByPath(instance interface{}, fieldPath string) (interface{}, bool) {
	valueOfIns := reflect.ValueOf(instance)
	fieldNames := strings.Split(fieldPath, ".")
	for _, name := range fieldNames {
		v := reflect.Indirect(valueOfIns)
		if v.Type().Kind() != reflect.Struct {
			return nil, false
		}
		v = v.FieldByName(name)
		if v.IsValid() {
			valueOfIns = v
		} else {
			return nil, false
		}

	}
	return valueOfIns.Interface(), true
}

func FieldPath2Index(instance interface{}, fieldPath string) (interface{}, []int, bool) {
	valueOfIns := reflect.ValueOf(instance)
	fieldNames := strings.Split(fieldPath, ".")
	indices := make([]int, 0, 4)
	for _, name := range fieldNames {
		v := reflect.Indirect(valueOfIns)
		if v.Type().Kind() != reflect.Struct {
			return nil, nil, false
		}

		if field, ok := v.Type().FieldByName(name); ok {
			indices = append(indices, field.Index[0])
			v = v.FieldByName(name)
			valueOfIns = v
		} else {
			return nil, nil, false
		}
	}
	return valueOfIns.Interface(), indices, true
}
