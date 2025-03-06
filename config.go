package bob

import (
	"github.com/xavi-group/bconf"
)

func NewConfig[TC, LC any](tracerConfig TC, loggerConfig LC) *Config[TC, LC] {
	return &Config[TC, LC]{
		TracerConfig: tracerConfig,
		LoggerConfig: loggerConfig,
	}
}

// Config defines the expected functions / values for configuring full application observability with logging and
// tracing. It is recommended to initialize a Config with bob.NewConfig().
type Config[TC, LC any] struct {
	TracerConfig TC
	LoggerConfig LC
}

// FieldSets combines FieldSets into a single list of FieldSets.
func FieldSets(fieldSets ...bconf.FieldSets) bconf.FieldSets {
	fullSet := bconf.FieldSets{}

	for _, group := range fieldSets {
		fullSet = append(fullSet, group...)
	}

	return fullSet
}
