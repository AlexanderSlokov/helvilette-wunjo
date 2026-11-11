package commands

// Ports lib/kamal/commands/auditor.rb and lock.rb at Kamal 2.12.0.

import (
	"encoding/base64"
	"path/filepath"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/tags"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

// Auditor builds the commands that write and read the audit log on a host.
type Auditor struct {
	Base
	details []tags.Tag
}

// NewAuditor returns an auditor that adds details, such as the role, to each line.
func NewAuditor(base Base, details ...tags.Tag) *Auditor {
	return &Auditor{Base: base, details: details}
}

// Record appends a tagged line to the audit log.
func (a *Auditor) Record(line string, details ...tags.Tag) Line {
	return combine("&&", makeDirectory(a.cfg.RunDirectory()),
		appendTo(w("echo", utils.EscapeShellValue(a.auditLine(line, details))), w(a.auditLogFile())))
}

// Reveal prints the last 50 lines of the audit log.
func (a *Auditor) Reveal() Line { return w("tail", "-n", "50", a.auditLogFile()) }

func (a *Auditor) auditLogFile() string {
	return filepath.Join(a.cfg.RunDirectory(), joinNonEmpty("-", a.cfg.Service(), a.cfg.Destination(), "audit.log"))
}

func (a *Auditor) auditLine(line string, details []tags.Tag) string {
	t := a.tags(append(append([]tags.Tag{}, a.details...), details...)...)
	return t.Except("version", "service_version", "service").String() + " " + line
}

func joinNonEmpty(sep string, parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, sep)
}

// Lock builds the commands of the deploy lock on the primary host.
type Lock struct{ Base }

// Acquire creates the lock directory and writes the lock details.
func (l Lock) Acquire(message, version string) Line {
	return combine("&&", w("mkdir", l.lockDir()), l.writeLockDetails(message, version))
}

// Release removes the lock.
func (l Lock) Release() Line {
	return combine("&&", w("rm", l.lockDetailsFile()), w("rm", "-r", l.lockDir()))
}

// Status prints the lock details when the lock exists.
func (l Lock) Status() Line {
	return combine("&&", writeTo(w("stat", l.lockDir()), w("/dev/null")), pipe(w("cat", l.lockDetailsFile()), w("base64", "-d")))
}

// EnsureLocksDirectory returns mkdir -p of the run directory.
func (l Lock) EnsureLocksDirectory() Line { return w("mkdir", "-p", l.cfg.RunDirectory()) }

func (l Lock) writeLockDetails(message, version string) Line {
	return writeTo(w("echo", `"`+encode64(l.lockDetails(message, version))+`"`), w(l.lockDetailsFile()))
}

func (l Lock) lockDir() string {
	return filepath.Join(l.cfg.RunDirectory(), joinNonEmpty("-", "lock", l.cfg.Service(), l.cfg.Destination()))
}

func (l Lock) lockDetailsFile() string { return filepath.Join(l.lockDir(), "details") }

func (l Lock) lockDetails(message, version string) string {
	return "Locked by: " + l.lockedBy() + " at " + l.rt.Now().UTC().Format("2006-01-02T15:04:05Z") +
		"\nVersion: " + version + "\nMessage: " + message
}

func (l Lock) lockedBy() string {
	name, err := l.rt.Git.UserName()
	if err != nil {
		return "Unknown"
	}
	return name
}

// encode64 follows Ruby's Base64.encode64: lines of 60 characters, each ending in a
// newline.
func encode64(text string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	var out strings.Builder
	for len(encoded) > 60 {
		out.WriteString(encoded[:60] + "\n")
		encoded = encoded[60:]
	}
	if encoded != "" {
		out.WriteString(encoded + "\n")
	}
	return out.String()
}
