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
	return []Option{wrapConsumer(defaultConsumer), wrapSettler(defaultSettler), wrapCleaner(defaultCleaner)}
}
