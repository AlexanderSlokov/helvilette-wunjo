package output

// Ports lib/kamal/output/otel_logger.rb at Kamal 2.12.0.

import (
	"fmt"
	"io"
	"strings"
)

// deployCommands are the commands that OTel events describe as deployments.
var deployCommands = map[string]bool{"deploy": true, "redeploy": true, "rollback": true, "setup": true}

// OtelLogger sends every line and a start and end event of each modifying command to
// an OTel collector.
type OtelLogger struct {
	endpoint string
	service  string
	shipper  *OtelShipper
	stdout   io.Writer
}

// NewOtelLogger returns a logger that ships with shipper and prints the endpoint
// to stdout when a command ends.
func NewOtelLogger(endpoint, service string, shipper *OtelShipper, stdout io.Writer) *OtelLogger {
	return &OtelLogger{endpoint: endpoint, service: service, shipper: shipper, stdout: stdout}
}

// Start sends the kamal.start event.
func (l *OtelLogger) Start(p Payload) error {
	attrs := append([]Attribute{StringAttr("kamal.command", fullCommand(p))}, l.deploymentAttrs(p, "")...)
	l.shipper.Event("kamal.start", InfoSeverity, attrs...)
	return nil
}

// Finish sends kamal.complete, or kamal.failed with the exception.
func (l *OtelLogger) Finish(p Payload, runtime float64) {
	attrs := []Attribute{StringAttr("kamal.command", fullCommand(p)), DoubleAttr("kamal.runtime", runtime)}
	if p.Exception != nil {
		attrs = append(attrs, StringAttr("exception.type", p.Exception.Class), StringAttr("exception.message", p.Exception.Message))
		l.shipper.Event("kamal.failed", ErrorSeverity, append(attrs, l.deploymentAttrs(p, "failed")...)...)
	} else {
		l.shipper.Event("kamal.complete", InfoSeverity, append(attrs, l.deploymentAttrs(p, "succeeded")...)...)
	}
	fmt.Fprintf(l.stdout, "Logs sent to %s\n", l.endpoint)
}

// Write ships one line with its host, stream, and level.
func (l *OtelLogger) Write(e Entry) { l.shipper.Append(e.Text, e.Host, e.IOStream, e.Severity) }

// Close stops the shipper and posts the buffered lines.
func (l *OtelLogger) Close() { l.shipper.Shutdown() }

// deploymentAttrs names the deployment for deploy, redeploy, rollback, and setup.
func (l *OtelLogger) deploymentAttrs(p Payload, status string) []Attribute {
	if !deployCommands[p.Command] {
		return nil
	}
	attrs := []Attribute{StringAttr("deployment.id", l.shipper.RunID()), StringAttr("deployment.name", fullCommand(p)+" "+l.service)}
	if status != "" {
		attrs = append(attrs, StringAttr("deployment.status", status))
	}
	return attrs
}

// fullCommand is "app boot" for `kamal app boot` and "deploy" for `kamal deploy`.
func fullCommand(p Payload) string { return strings.Join(present(p.Command, p.Subcommand), " ") }
