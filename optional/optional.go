package optional

type Optional interface {
	Get() interface{}
	IsNone() bool
}

type None struct{}

func (o None) Get() interface{} { return struct{}{} }
func (o None) IsNone() bool     { return true }

type Some struct {
	Value interface{}
}

func (o Some) Get() interface{} { return o.Value }
func (o Some) IsNone() bool     { return false }
func (o Some) Some(receiver *interface{}) {
	*receiver = o.Value
}
