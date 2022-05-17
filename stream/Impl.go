package stream

import (
	"github.com/cornelk/hashmap"
	"github.com/emirpasic/gods/maps/treemap"
	"github.com/emirpasic/gods/utils"
	"github.com/kabu1204/go-pipeline/optional"
	"github.com/kabu1204/go-pipeline/types"
	"github.com/panjf2000/ants/v2"
	"reflect"
	"sync"
	"sync/atomic"
)

// source <- Filtered <- ToSlice

//type settlerOption func(this *stream)
type Option func(*stream)
type wrapperType func(next *stream) []Option

type stream struct {
	source    types.Iterator
	prev      *stream
	wrapper   wrapperType
	consumer  types.Consumer
	settler   func(size int64, opts ...Option)
	cleaner   func()
	canceller func() bool
	parallel  int
	Name      string
}

func (s *stream) terminate() {
	head := s.setFunctor()
	it := s.source
	head.settler(int64(it.Len()))
	for v, ok := it.Next(); ok && !head.canceller(); v, ok = it.Next() {
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

func wrapConsumer(c types.Consumer) Option        { return func(s *stream) { s.consumer = c } }
func wrapSettler(c func(int64, ...Option)) Option { return func(s *stream) { s.settler = c } }
func wrapCleaner(c func()) Option                 { return func(s *stream) { s.cleaner = c } }
func wrapCanceller(c func() bool) Option          { return func(s *stream) { s.canceller = c } }

func (s *stream) setFunctor() *stream {
	s.unwrap(&stream{
		source:    s.source,
		prev:      s,
		consumer:  func(_ interface{}) {},
		settler:   func(_ int64, _ ...Option) {},
		cleaner:   func() {},
		canceller: func() bool { return false },
		parallel:  0,
		Name:      "DummyTail",
	})
	p := s
	for ; p.prev != nil; p = p.prev {
		p.prev.unwrap(p)
	}
	return p
}

func newStream(prev *stream, wrapper wrapperType, name string) *stream {
	return &stream{
		source:   prev.source,
		prev:     prev,
		wrapper:  wrapper,
		parallel: 0,
		Name:     name,
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
			next.consumeOne(e)
		}
		return append(defaultWrapper(next), wrapConsumer(consumer))
	}
	return newStream(s, wrapper, "Peek")
}

func (s *stream) Parallel(n int) Stream {
	wrapper := func(next *stream) []Option {
		var wg sync.WaitGroup
		var pool *ants.Pool
		settler := func(sz int64, opts ...Option) {
			if n >= 0 {
				toggleParallel := func(this *stream) { this.parallel = n }
				opts = append(opts, toggleParallel)
			}
			for _, o := range opts {
				o(next.prev)
			}
			pool, _ = ants.NewPool(MaxInt(n, 1))
			next.settler(sz, opts...)
		}
		consumer := func(e interface{}) {
			wg.Add(1)
			f := func() {
				next.consumeOne(e)
				wg.Done()
			}
			_ = pool.Submit(f)
		}
		cleaner := func() {
			wg.Wait()
			pool.Release()
			pool = nil
			next.cleaner()
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer), wrapCleaner(cleaner))
	}
	return newStream(s, wrapper, "Parallel")
}

// stateful

func (s *stream) Distinct(f types.IntFunction) Stream {
	wrapper := func(next *stream) []Option {
		var set *hashmap.HashMap
		settler := func(sz int64, opts ...Option) {
			for _, o := range opts {
				o(next.prev)
			}
			set = &hashmap.HashMap{}
			next.settler(sz, opts...)
		}
		consumer := func(e interface{}) {
			hash := f(e)
			if _, exist := set.GetOrInsert(hash, struct{}{}); !exist {
				next.consumeOne(e)
			}
		}
		cleaner := func() {

			next.cleaner()
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer), wrapCleaner(cleaner))
	}
	return newStream(s, wrapper, "Distinct")
}

func (s *stream) Sorted(cmp types.Comparator, keepParallel bool) Stream {
	wrapper := func(next *stream) []Option {
		var buffer chan interface{}
		var mp *treemap.Map
		this := next.prev
		settler := func(capacity int64, opts ...Option) {
			for _, o := range opts {
				o(this)
			}
			mp = treemap.NewWith(utils.Comparator(cmp))
			if this.parallel > 0 {
				buffer = make(chan interface{}, capacity)
				writer := func() {
					for e := range buffer {
						if c, ok := mp.Get(e); ok {
							mp.Put(e, c.(int)+1)
						} else {
							mp.Put(e, 1)
						}
					}
				}
				go writer()
			}
			next.settler(capacity, opts...)
		}
		consumer := func(e interface{}) {
			if this.parallel > 0 {
				buffer <- e
			} else {
				if c, ok := mp.Get(e); ok {
					mp.Put(e, c.(int)+1)
				} else {
					mp.Put(e, 1)
				}
			}
		}
		cleaner := func() {
			opts := make([]Option, 0)
			if !keepParallel || this.parallel == 0 {
				opts = append(opts, func(_this *stream) { _this.parallel = 0 })
			}
			if this.parallel > 0 {
				close(buffer)
			}
			next.settler(int64(mp.Size()), opts...)
			it := mp.Iterator()
			for it.Next() {
				e, c := it.Key(), it.Value().(int)
				for ; c > 0; c-- {
					next.consumeOne(e) // 值传递？
				}
			}
			mp.Clear()
			mp = nil
			next.cleaner()
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer), wrapCleaner(cleaner))
	}
	if keepParallel {
		return newStream(s, wrapper, "Sorted").Parallel(-1)
	}
	return newStream(s, wrapper, "Sorted")
}

func (s *stream) Limit(N int64) Stream {
	wrapper := func(next *stream) []Option {
		var cnt *int64
		settler := func(sz int64, opts ...Option) {
			for _, o := range opts {
				o(next.prev)
			}
			cnt = new(int64)
			next.settler(sz, opts...)
		}
		consumer := func(e interface{}) {
			for old := atomic.LoadInt64(cnt); old < N; old = atomic.LoadInt64(cnt) {
				if atomic.CompareAndSwapInt64(cnt, old, old+1) {
					next.consumeOne(e)
					break
				}
			}
		}
		cleaner := func() {
			atomic.StoreInt64(cnt, N)
			cnt = nil
			next.cleaner()
		}
		canceller := func() bool {
			return atomic.LoadInt64(cnt) == N
		}
		return append(defaultWrapper(next), wrapSettler(settler),
			wrapConsumer(consumer), wrapCleaner(cleaner), wrapCanceller(canceller))
	}
	return newStream(s, wrapper, "Limit")
}

func (s *stream) Skip(N int64) Stream {
	wrapper := func(next *stream) []Option {
		var cnt *int64
		settler := func(sz int64, opts ...Option) {
			for _, o := range opts {
				o(next.prev)
			}
			cnt = new(int64)
			next.settler(sz, opts...)
		}
		consumer := func(e interface{}) {
			for old := atomic.LoadInt64(cnt); old < N; old = atomic.LoadInt64(cnt) {
				if atomic.CompareAndSwapInt64(cnt, old, old+1) {
					return
				}
			}
			next.consumeOne(e)
		}
		cleaner := func() {
			atomic.StoreInt64(cnt, N)
			cnt = nil
			next.cleaner()
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer), wrapCleaner(cleaner))
	}
	return newStream(s, wrapper, "Skip")
}

// termination

func (s *stream) ToSlice() types.Slice {
	var slice types.Slice
	wrapper := func(next *stream) []Option {
		settler := func(sz int64, opts ...Option) {
			for _, o := range opts {
				o(next.prev)
			}
			slice = make(types.Slice, 0, sz)
		}
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
		settler := func(sz int64, opts ...Option) {
			for _, o := range opts {
				o(next.prev)
			}
			slice = reflect.MakeSlice(sliceTyp, 0, int(sz))
		}
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
		settler := func(sz int64, opts ...Option) {
			for _, o := range opts {
				o(next.prev)
			}
			flag = true
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			if !p(e) {
				flag = false
			}
		}
		canceller := func() bool {
			return !flag
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer), wrapCanceller(canceller))
	}
	newStream(s, wrapper, "AllMatch").terminate()
	return flag
}

func (s *stream) NoneMatch(p types.Predicate) bool {
	var flag bool
	wrapper := func(next *stream) []Option {
		settler := func(sz int64, opts ...Option) {
			for _, o := range opts {
				o(next.prev)
			}
			flag = false
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			if !p(e) {
				flag = true
			}
		}
		canceller := func() bool {
			return flag
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer), wrapCanceller(canceller))
	}
	newStream(s, wrapper, "NoneMatch").terminate()
	return flag
}

func (s *stream) AnyMatch(p types.Predicate) bool {
	var flag bool
	wrapper := func(next *stream) []Option {
		settler := func(sz int64, opts ...Option) {
			for _, o := range opts {
				o(next.prev)
			}
			flag = false
			next.settler(sz)
		}
		consumer := func(e interface{}) {
			if p(e) {
				flag = true
			}
		}
		canceller := func() bool {
			return flag
		}
		return append(defaultWrapper(next), wrapSettler(settler), wrapConsumer(consumer), wrapCanceller(canceller))
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

func (s *stream) FindFirst() optional.Optional {
	none := true
	var result interface{}
	wrapper := func(next *stream) []Option {
		consumer := func(e interface{}) {
			if none {
				result = e
				none = false
			}
		}
		canceller := func() bool {
			return !none
		}
		return append(defaultWrapper(next), wrapConsumer(consumer), wrapCanceller(canceller))
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
