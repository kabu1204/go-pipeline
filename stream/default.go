package stream

func defaultWrapper(next *stream) []Option {
	defaultConsumer := func(e interface{}) {
		next.consumeOne(e)
	}
	defaultSettler := func(capacity int64, opts ...Option) {
		for _, o := range opts {
			o(next.prev) // o(this)
		}
		next.settler(capacity, opts...)
	}
	defaultCleaner := func() {
		next.cleaner()
	}
	defaultCanceller := func() bool {
		return next.canceller()
	}
	return []Option{wrapConsumer(defaultConsumer), wrapSettler(defaultSettler),
		wrapCleaner(defaultCleaner), wrapCanceller(defaultCanceller)}
}
