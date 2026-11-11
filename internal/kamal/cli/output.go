package cli

// Ports Kamal::Cli::Base#modify and #say, and the modify, log, and
// configure_output_with parts of lib/kamal/commander.rb at Kamal 2.12.0.

import (
	"errors"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/configuration"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/output"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// modify runs fn as a modifying command. The outermost call tells the output
// loggers that the command starts and ends, and closes them at the end. lock takes
// the deploy lock around fn.
func (c *CLI) modify(lock bool, fn func() error) error {
	c.logging = true
	c.modifyDepth++
	defer c.modifyFinished()
	run := fn
	if lock {
		run = func() error { return c.withLock(fn) }
	}
	if c.modifyDepth > 1 {
		return run()
	}
	return c.instrumentModify(run)
}

func (c *CLI) modifyFinished() {
	c.modifyDepth--
	if c.modifyDepth == 0 {
		c.logs.Close()
	}
}

// instrumentModify surrounds fn with the start and finish events, as
// ActiveSupport::Notifications.instrument("modify.kamal") does. A failure, or a
// panic that Run reports, sets the exception of the finish event.
func (c *CLI) instrumentModify(fn func() error) (err error) {
	payload := c.modifyPayload()
	if err := c.logs.Start(payload); err != nil {
		return err
	}
	defer func() {
		r := recover()
		if failure, ok := r.(error); ok {
			payload.Exception = exceptionFor(reportable(failure))
		} else if err != nil {
			payload.Exception = exceptionFor(err)
		}
		c.logs.Finish(payload)
		if r != nil {
			panic(r)
		}
	}()
	return fn()
}

func (c *CLI) modifyPayload() output.Payload {
	payload := output.Payload{Destination: c.config().Destination()}
	if len(c.first) > 0 {
		payload.Command = c.first[0]
	}
	if len(c.first) > 1 {
		payload.Subcommand = c.first[1]
	}
	return payload
}

// exceptionFor names err as the Ruby exception that Kamal would raise.
func exceptionFor(err error) *output.Exception {
	var exec *sshexec.ExecuteError
	if errors.As(err, &exec) {
		return &output.Exception{Class: "SSHKit::Runner::ExecuteError", Message: err.Error()}
	}
	return &output.Exception{Class: errorClass(err), Message: err.Error()}
}

// logSaid sends a message that say printed to the loggers, once a modifying
// command has started, as KAMAL.log does.
func (c *CLI) logSaid(message string) {
	if c.logging {
		c.logs.Write(output.Entry{Text: message + "\n", Severity: output.NoSeverity})
	}
}

// configureOutput adds a logger for each destination in the output section and
// sends every formatted line to them.
func (c *CLI) configureOutput(cfg *configuration.Configuration) {
	settings := cfg.Output()
	if !settings.Enabled() {
		return
	}
	for _, kind := range settings.Kinds() {
		values, _ := settings.Settings(kind)
		c.logs.Add(c.newOutputLogger(kind, values, cfg))
	}
	c.out.Sink = func(line sshexec.LogLine) {
		c.logs.Write(output.Entry{Text: line.Text, Host: line.Host, IOStream: line.Stream, Severity: output.Severity(line.Level)})
	}
}

func (c *CLI) newOutputLogger(kind string, values *yamlord.Ordered, cfg *configuration.Configuration) output.Logger {
	if kind == "file" {
		return output.NewFileLogger(values.At("path").ToS(), c.rt.OutputFiles, c.rt.Now, c.rt.Stdout)
	}
	endpoint := values.At("endpoint").ToS()
	shipper := output.NewOtelShipper(endpoint, c.otelTags(cfg),
		output.ShipperDeps{Now: c.rt.Now, Poster: c.rt.OTelPoster, Stderr: c.rt.Stderr, Version: Version})
	shipper.Start()
	return output.NewOtelLogger(endpoint, cfg.Service(), shipper, c.rt.Stdout)
}

// otelTags are Kamal::Tags.from_config without service_version and recorded_at.
func (c *CLI) otelTags(cfg *configuration.Configuration) tags.Tags {
	deploy := tags.Deploy{Destination: cfg.Destination(), Version: cfg.Version(), Service: cfg.Service(), AbbreviatedVersion: cfg.AbbreviatedVersion()}
	return tags.Default(deploy, c.rt.Now(), c.kamal.Performer()).Except("service_version", "recorded_at")
}
