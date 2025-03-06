package bob

import (
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func NewObserver(name string, packageName string) Observer {
	return &otelZapObserver{
		tracer: singletonTraceProvider.Tracer(name),
		logger: otelzap.New(zap.L().Named(name)),
	}
}

type Observer interface {
	Logger() *otelzap.Logger
	Tracer() trace.Tracer
}

type otelZapObserver struct {
	tracer      trace.Tracer
	logger      *otelzap.Logger
	packageName string
}

func (o *otelZapObserver) Logger() *otelzap.Logger {
	return o.logger
}

func (o *otelZapObserver) Tracer() trace.Tracer {
	return o.tracer
}
