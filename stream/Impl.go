package stream

import (
	"github.com/kabu1204/go-pipeline/optional"
	"github.com/kabu1204/go-pipeline/types"
	"reflect"
	"sort"
)

// source <- Filtered <- ToSlice

type Option func(*stream)
type wrappedConsumer func(next *stream) []Option

type stream struct {
	source   types.Iterator
	prev     *stream
	wrapper  wrappedConsumer
	consumer types.Consumer
	settler  func(int64)
	cleaner  func()
	Name     string
}

func (s *stream) terminate() {
	head := s.setFunctor()
	it := s.source
	head.settler(10)
	for v, ok := it.Next(); ok; v, ok = it.Next() {
		head.consumeOne(*v)
	}
	head.cleaner()
}

func (s *stream) consumeOne(e interface{}) {
	s.consumer(e)
}

func (s *stream) unwrap(next *stream) {
	opts := s.wrapper(next)
	for _, o := range opts {
		o(s)
	}
}

func wrapConsumer(c types.Consumer) Option { return func(s *stream) { s.consumer = c } }
func wrapSettler(c func(int64)) Option     { return func(s *stream) { s.settler = c } }
func wrapCleaner(c func()) Option          { return func(s *stream) { s.cleaner = c } }

func (s *stream) setFunctor() *stream {
	s.unwrap(&stream{
		source:   s.source,
		prev:     s,
		consumer: func(e interface{}) {},
		settler:  func(i int64) {},
		cleaner:  func() {},
		Name:     "DummyTail",
	})
	p := s
	for ; p.prev != nil; p = p.prev {
		p.prev.unwrap(p)
	}
	return p
}

func newStream(prev *stream, wrapper wrappedConsumer, name string) *stream {
	return &stream{
		source:  prev.source,
		prev:    prev,
		wrapper: wrapper,
		Name:    name,
	}
}

// stateless

func (s *stream) Filter(p types.Predicate) Stream {
	// s is prev
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			if p(e) {
				next.consumeOne(e)
			}
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	return newStream(s, wrapper, "Filter")
}

func (s *stream) Map(f types.Function) Stream {
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			next.consumeOne(f(e))
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	return newStream(s, wrapper, "Map")
}

func (s *stream) MapField(fieldPath string) Stream {
	wrapper := func(next *stream) []Option {
		var indices []int
		var ok bool
		var t interface{}
		consumer := func(e interface{}) {
			if indices != nil {
				next.consumeOne(reflect.Indirect(reflect.ValueOf(e)).FieldByIndex(indices).Interface())
			} else if t, indices, ok = types.FieldPath2Index(e, fieldPath); ok {
				next.consumeOne(t)
			} else {
				panic("Field path is INCORRECT.")
			}
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	return newStream(s, wrapper, "MapField")
}

func (s *stream) FlatMap(f func(interface{}) Stream) Stream {
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			stm := f(e)
			stm.ForEach(next.consumeOne)
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	return newStream(s, wrapper, "FlatMap")
}

func (s *stream) Peek(f types.Consumer) Stream {
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			f(e)
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	return newStream(s, wrapper, "Peek")
}

// stateful

func (s *stream) Distinct(f types.IntFunction) Stream {
	wrapper := func(next *stream) []Option {
		var set map[int]struct{}
		settler := func(sz int64) {
			set = make(map[int]struct{})
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			hash := f(e)
			if _, exist := set[hash]; !exist {
				set[hash] = struct{}{}
				next.consumeOne(e)
			}
		}
		cleaner := func() {
			set = nil
			next.cleaner()
		}
		return []Option{wrapSettler(settler), wrapConsumer(consumer), wrapCleaner(cleaner)}
	}
	return newStream(s, wrapper, "Distinct")
}

func (s *stream) Sorted(cmp types.Comparator) Stream {
	wrapper := func(next *stream) []Option {
		var slice types.Slice
		settler := func(capacity int64) {
			slice = make(types.Slice, 0, capacity)
			next.settler(capacity)
		}
		consumer := func(e interface{}) {
			slice = append(slice, e)
		}
		cleaner := func() {
			sort.Sort(&types.Array{Data: slice, Cmp: cmp})
			next.settler(int64(len(slice)))
			it := slice.Iterator()
			for v, ok := it.Next(); ok; v, ok = it.Next() {
				next.consumeOne(*v)
			}
			slice = nil
			next.cleaner()
		}
		return []Option{wrapSettler(settler), wrapConsumer(consumer), wrapCleaner(cleaner)}
	}
	return newStream(s, wrapper, "Sorted")
}

func (s *stream) Limit(N int64) Stream {
	wrapper := func(next *stream) []Option {
		var cnt int64
		settler := func(sz int64) {
			cnt = 0
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			if cnt < N {
				next.consumeOne(e)
				cnt++
			}
			// TODO: early stop
		}
		cleaner := func() {
			cnt = 0
			next.cleaner()
		}
		return []Option{wrapSettler(settler), wrapConsumer(consumer), wrapCleaner(cleaner)}
	}
	return newStream(s, wrapper, "Limit")
}

func (s *stream) Skip(N int64) Stream {
	wrapper := func(next *stream) []Option {
		var cnt int64
		settler := func(sz int64) {
			cnt = 0
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			if cnt < N {
				cnt++
			} else {
				next.consumeOne(e)
			}
		}
		cleaner := func() {
			cnt = 0
			next.cleaner()
		}
		return []Option{wrapSettler(settler), wrapConsumer(consumer), wrapCleaner(cleaner)}
	}
	return newStream(s, wrapper, "Skip")
}

// termination

func (s *stream) ToSlice() types.Slice {
	var slice types.Slice
	wrapper := func(next *stream) []Option {
		settler := func(sz int64) { slice = make(types.Slice, 0, sz) }
		consumer := func(e interface{}) {
			slice = append(slice, e)
		}
		return append(defaultWrapper(next), wrapConsumer(consumer), wrapSettler(settler))
	}
	newStream(s, wrapper, "ToSlice").terminate()
	return slice
}

func (s *stream) ToSliceLike(some interface{}) interface{} {
	return s.ToSliceOf(reflect.TypeOf(some))
}

func (s *stream) ToSliceOf(typ reflect.Type) interface{} {
	sliceTyp := reflect.SliceOf(typ)
	var slice reflect.Value
	wrapper := func(next *stream) []Option {
		settler := func(sz int64) { slice = reflect.MakeSlice(sliceTyp, 0, int(sz)) }
		consumer := func(e interface{}) {
			slice = reflect.Append(slice, reflect.ValueOf(e))
		}
		return append(defaultWrapper(next), wrapConsumer(consumer), wrapSettler(settler))
	}
	newStream(s, wrapper, "ToSliceOf").terminate()
	return slice.Interface()
}

func (s *stream) ForEach(f types.Consumer) {
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) { f(e) }
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "ForEach").terminate()
}

func (s *stream) AllMatch(p types.Predicate) bool {
	var flag bool
	wrapper := func(next *stream) []Option {
		settler := func(sz int64) {
			flag = true
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			if !p(e) {
				flag = false
				// TODO: early stop
			}
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "AllMatch").terminate()
	return flag
}

func (s *stream) NoneMatch(p types.Predicate) bool {
	var flag bool
	wrapper := func(next *stream) []Option {
		settler := func(sz int64) {
			flag = false
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			if !p(e) {
				flag = true
				// TODO: early stop
			}
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "NoneMatch").terminate()
	return flag
}

func (s *stream) AnyMatch(p types.Predicate) bool {
	var flag bool
	wrapper := func(next *stream) []Option {
		settler := func(sz int64) {
			flag = false
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			if p(e) {
				flag = true
				// TODO: early stop
			}
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "AnyMatch").terminate()
	return flag
}

func (s *stream) Reduce(accumulator types.BinaryOperator) optional.Optional {
	var result interface{}
	none := true
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			if none {
				result = e
				none = false
			} else {
				result = accumulator(result, e)
			}
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "Reduce").terminate()
	if none {
		return optional.None{}
	}
	return optional.Some{Value: result}
}

func (s *stream) ReduceFrom(initValue interface{}, accumulator types.BinaryOperator) interface{} {
	result := initValue
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			result = accumulator(result, e)
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "ReduceFrom").terminate()
	return result
}

func (s *stream) ReduceWith(initValue types.R, accumulator func(types.R, types.T) types.R) types.R {
	result := initValue
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			result = accumulator(result, e)
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "ReduceWith").terminate()
	return result
}

func (s *stream) FindFirst(p types.Predicate) optional.Optional {
	none := true
	var result interface{}
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			if none {
				result = e
				none = false
			}
			// TODO: early stop
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "FindFirst").terminate()
	if none {
		return optional.None{}
	}
	return optional.Some{Value: result}
}

func (s *stream) FindFirstMatch(p types.Predicate) optional.Optional {
	none := true
	var result interface{}
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			if none && p(e) {
				result = e
				none = false
			}
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "FindFirstMatch").terminate()
	if none {
		return optional.None{}
	}
	return optional.Some{Value: result}
}

func (s *stream) Count() int64 {
	var cnt int64 = 0
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) { cnt++ }
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	newStream(s, wrapper, "Count").terminate()
	return cnt
}
