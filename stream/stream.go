package stream

import (
	"code.byted.org/yuchengye/pipeline/optional"
	"code.byted.org/yuchengye/pipeline/types"
	"reflect"
)

type Stream interface {
	// stateless (nothing to do with elements order)
	Filter(p types.Predicate) Stream
	Map(f types.Function) Stream
	MapField(fieldPath string) Stream
	FlatMap(f func(interface{}) Stream) Stream
	Peek(f types.Consumer) Stream

	// stateless (nothing to do with elements order)
	Distinct(f types.IntFunction) Stream // custom hash, therefore the elements order may affect result
	Sorted(cmp types.Comparator) Stream  // stable, therefore the elements order may affect sort result
	Limit(N int64) Stream                // first N elems
	Skip(N int64) Stream                 // skip first N elems

	ForEach(f types.Consumer)
	ToSlice() types.Slice
	ToSliceLike(some interface{}) interface{}
	ToSliceOf(typ reflect.Type) interface{}
	AllMatch(p types.Predicate) bool
	NoneMatch(p types.Predicate) bool
	AnyMatch(p types.Predicate) bool
	Reduce(accumulator types.BinaryOperator) optional.Optional
	ReduceFrom(initValue interface{}, accumulator types.BinaryOperator) interface{}
	ReduceWith(initValue types.R, accumulator func(types.R, types.T) types.R) types.R
	FindFirst(p types.Predicate) optional.Optional
	FindFirstMatch(p types.Predicate) optional.Optional
	Count() int64
}
