package configuration

// Ports lib/kamal/configuration/logging.rb at Kamal 2.12.0.

import (
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Logging holds the Docker log driver and its options.
type Logging struct {
	config *yamlord.Ordered
}

func newLogging(config *yamlord.Value, context string) (*Logging, error) {
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if err := (validator{context: context}).againstExample(config, example("logging")); err != nil {
		return nil, err
	}
	return &Logging{config: config.Map()}, nil
}

// Driver returns the log driver, or "" when unset.
func (l *Logging) Driver() string { return l.config.At("driver").ToS() }

// Options returns the log options.
func (l *Logging) Options() *yamlord.Ordered {
	if options := l.config.At("options").Map(); options != nil {
		return options
	}
	return yamlord.NewOrdered()
}

// Merge deep-merges other over l.
func (l *Logging) Merge(other *Logging) *Logging {
	return &Logging{config: l.config.DeepMerge(other.config)}
}

// Args returns --log-driver and --log-opt flags, or --log-opt max-size="10m" when
// nothing is set.
func (l *Logging) Args() shell.Line {
	driver := l.config.At("driver")
	if !driver.Present() && !l.config.At("options").Present() {
		return utils.Argumentize("--log-opt", []utils.Attr{{Key: "max-size", Value: yamlord.NewString("10m")}}, false)
	}
	var line shell.Line
	if !driver.IsNull() {
		line = utils.Optionize([]utils.Attr{{Key: "log-driver", Value: driver}})
	}
	return shell.Concat(line, utils.Argumentize("--log-opt", utils.AttrsOf(l.Options()), false))
}
