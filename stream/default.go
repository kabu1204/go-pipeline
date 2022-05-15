package stream

func defaultWrapper(next *stream) []Option {
	defaultConsumer := func(e interface{}) {
		next.consumeOne(e)
	}
	defaultSettler := func(capacity int64) {
		next.settler(capacity)
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
