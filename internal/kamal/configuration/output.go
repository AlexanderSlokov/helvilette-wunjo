package configuration

// Ports lib/kamal/configuration/output.rb at Kamal 2.12.0. The loggers it builds
// belong to the output stage of the port.

import (
	"errors"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Kamal raises these as ArgumentError when it builds the loggers during
// configuration, so the port checks the same settings at the same point.
var (
	ErrOtelEndpointRequired = errors.New("OTel endpoint is required")
	ErrFilePathRequired     = errors.New("file path is required")
)

// Output holds the settings of the deploy log destinations, such as file and otel.
type Output struct {
	config *yamlord.Ordered
}

func newOutput(config *yamlord.Value) (*Output, error) {
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if config.Map().Len() > 0 {
		if err := (validator{context: "output"}).againstExample(config, example("output")); err != nil {
			return nil, err
		}
	}
	o := &Output{config: config.Map()}
	return o, o.checkLoggerSettings()
}

func (o *Output) checkLoggerSettings() error {
	required := map[string]struct {
		key string
		err error
	}{"otel": {"endpoint", ErrOtelEndpointRequired}, "file": {"path", ErrFilePathRequired}}
	for _, kind := range o.Kinds() {
		settings, _ := o.Settings(kind)
		if !settings.At(required[kind].key).Truthy() {
			return required[kind].err
		}
	}
	return nil
}

// Enabled reports whether any destination is set.
func (o *Output) Enabled() bool { return o.config.Len() > 0 }

// ToH returns the raw settings.
func (o *Output) ToH() *yamlord.Ordered { return o.config }

// Settings returns the settings of one destination, such as "otel", and whether it
// is set. Kamal builds a logger for each "otel" and "file" key in file order.
func (o *Output) Settings(kind string) (*yamlord.Ordered, bool) {
	value, ok := o.config.Get(kind)
	if !ok {
		return nil, false
	}
	if value.Map() == nil {
		return yamlord.NewOrdered(), true
	}
	return value.Map(), true
}

// Kinds returns the configured destinations that Kamal knows, in file order.
func (o *Output) Kinds() []string {
	var kinds []string
	for _, key := range o.config.Keys() {
		if key == "otel" || key == "file" {
			kinds = append(kinds, key)
		}
	}
	return kinds
}
