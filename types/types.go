package types

type (
	T interface{}

	R interface{}

	Slice []interface{}

	Map map[interface{}]interface{}

	Predicate func(interface{}) bool

	Function func(interface{}) interface{}

	Consumer func(interface{})

	IntFunction func(interface{}) int

	Comparator func(e1, e2 interface{}) int

	BinaryOperator func(e1, e2 interface{}) interface{}
)

type Array struct {
	Data Slice
	Cmp  Comparator
}

// func(){
//	next.xxx()
//}
