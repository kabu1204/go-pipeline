package types

type Iterator interface {
	hasNext() bool
	Next() (*interface{}, bool)
	Len() int // for slice and map: a definite number; for channel: -1
}

type sliceIterator struct {
	index int
	slice *Slice
}

func (s *Slice) Iterator() *sliceIterator {
	return &sliceIterator{
		index: -1,
		slice: s,
	}
}

func (it *sliceIterator) hasNext() bool {
	return it.index < len(*it.slice)-1
}

func (it *sliceIterator) Next() (*interface{}, bool) {
	if it.hasNext() {
		it.index++
		return &((*it.slice)[it.index]), true
	}
	return nil, false
}

func (it *sliceIterator) Len() int {
	return len(*it.slice)
}

func (it *sliceIterator) At(i int) *interface{} {
	return &((*it.slice)[i])
}

func (it *sliceIterator) Seek(i int) bool {
	if i < 0 || i >= len(*it.slice) {
		return false
	}
	it.index = i
	return true
}

type mapIterator struct {
	f func() (interface{}, bool)
	m *Map
}
