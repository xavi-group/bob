package bob

import (
	"fmt"
	"slices"

	"github.com/xavi-group/bconf"
)

const (
	logFieldSetKey  = "log"
	otelFieldSetKey = "otel"
	otlpFieldSetKey = "otlp"

	otelExportersKey     = "exporters"
	otelConsoleFormatKey = "console_format"

	otlpEndpointKindKey = "endpoint_kind"
	otlpHostKey         = "host"
	otlpPortKey         = "port"

	logColorKey  = "color"
	logConfigKey = "config"
	logFormatKey = "format"
	logLevelKey  = "level"
)

// Config defines the expected functions / values for configuring a full application monitor with logging and tracing.
type Config struct {
	bconf.ConfigStruct
	TracerConfig *TracerConfig
	LoggerConfig *LoggerConfig
}

// LoggerConfig defines the expected values for configuring an application logger.
type LoggerConfig struct {
	bconf.ConfigStruct
	AppID     string `bconf:"app.id"`
	LogColor  bool   `bconf:"log.color"`
	LogConfig string `bconf:"log.config"`
	LogFormat string `bconf:"log.format"`
	LogLevel  string `bconf:"log.level"`
}

// TracerConfig defines the expected values for configuring an application tracer.
type TracerConfig struct {
	bconf.ConfigStruct
	AppID             string   `bconf:"app.id"`
	AppName           string   `bconf:"app.name"`
	OtelExporters     []string `bconf:"otel.exporters"`
	OtelConsoleFormat string   `bconf:"otel.console_format"`
	OtlpEndpointKind  string   `bconf:"otlp.endpoint_kind"`
	OtlpHost          string   `bconf:"otlp.host"`
	OtlpPort          int      `bconf:"otlp.port"`
}

// FieldSets defines the field-sets for a full application monitor with logging and tracing.
func FieldSets() bconf.FieldSets {
	return bconf.FieldSets{
		LoggerFieldSet(),
		OtelFieldSet(),
		OtlpFieldSet(),
	}
}

// TracerFieldSets defines the field-sets for an application tracer.
func TracerFieldSets() bconf.FieldSets {
	return bconf.FieldSets{
		OtelFieldSet(),
		OtlpFieldSet(),
	}
}

// LoggerFieldSets defines the field-sets for an applicaiton logger.
func LoggerFieldSets() bconf.FieldSets {
	return bconf.FieldSets{
		LoggerFieldSet(),
	}
}

// LoggerFieldSet defines the field-set for an application logger.
func LoggerFieldSet() *bconf.FieldSet {
	return bconf.FSB(logFieldSetKey).Fields(
		bconf.FB(logColorKey, bconf.Bool).Default(true).C(),
		bconf.FB(logConfigKey, bconf.String).Default("production").Enumeration("production", "development").C(),
		bconf.FB(logFormatKey, bconf.String).Default("json").Enumeration("console", "json").C(),
		bconf.FB(logLevelKey, bconf.String).Default("info").
			Enumeration("debug", "info", "warn", "error", "dpanic", "panic", "fatal").C(),
	).C()
}

// OtelFieldSet ...
func OtelFieldSet() *bconf.FieldSet {
	return bconf.FSB(otelFieldSetKey).Fields(
		bconf.FB(otelExportersKey, bconf.Strings).Default([]string{"console"}).Validator(
			func(v any) error {
				acceptedValues := []string{"console", "otlp"}

				fieldValues, ok := v.([]string)
				if !ok {
					return fmt.Errorf("unexpected field-value type provided to validator")
				}

				for _, value := range fieldValues {
					if found := slices.Contains(acceptedValues, value); !found {
						return fmt.Errorf("invalid exporter value: '%s'", value)
					}
				}

				return nil
			},
		).C(),
		bconf.FB(otelConsoleFormatKey, bconf.String).Default("production").Enumeration("production", "pretty").C(),
	).C()
}

// OtlpFieldSet ...
func OtlpFieldSet() *bconf.FieldSet {
	return bconf.FSB(otlpFieldSetKey).Fields(
		bconf.FB(otlpEndpointKindKey, bconf.String).Default("agent").Enumeration("agent", "collector").C(),
		bconf.FB(otlpHostKey, bconf.String).Required().C(),
		bconf.FB(otlpPortKey, bconf.Int).Default(6831).C(),
	).LoadConditions(
		bconf.LCB(
			func(f bconf.FieldValueFinder) (bool, error) {
				exporters, found, err := f.GetStrings(otelFieldSetKey, otelExportersKey)
				if !found || err != nil {
					return false, fmt.Errorf("problem getting exporters field value")
				}

				otlpExporterFound := false
				for _, exporter := range exporters {
					if exporter == "otlp" {
						otlpExporterFound = true

						break
					}
				}

				return otlpExporterFound, nil
			},
		).AddFieldSetDependencies(otelFieldSetKey, otelExportersKey).C(),
	).C()
}
