package bob

func NewObserver[T, L any](tracer T, logger L) *Observer[T, L] {
	return &Observer[T, L]{
		tracer: tracer,
		logger: logger,
	}
}

type Observer[T, L any] struct {
	tracer T
	logger L
}

func (o *Observer[T, L]) Tracer() T {
	return o.tracer
}

func (o *Observer[T, L]) Logger() L {
	return o.logger
}
