// Package envfile writes environment variables in the format of `docker run
// --env-file`, with secret values already looked up.
//
// Ports lib/kamal/env_file.rb at Kamal 2.12.0.
package envfile

import (
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/utils"
)

// Var is one variable of an env file.
type Var struct {
	Key   string
	Value string
}

// Render returns the env file text, one KEY=value line per variable. An empty list
// renders as one newline, so SSH uploads never send an empty file.
//
//	envfile.Render([]envfile.Var{{"RAILS_MASTER_KEY", "abc"}}) // "RAILS_MASTER_KEY=abc\n"
func Render(vars []Var) string {
	var out strings.Builder
	for _, v := range vars {
		out.WriteString(v.Key + "=" + escapeValue(v.Value) + "\n")
	}
	if out.Len() == 0 {
		return "\n"
	}
	return out.String()
}

// escapeValue keeps non-ASCII runs as they are and escapes ASCII runs as Ruby's
// String#dump does, without the surrounding quotes. Docker reads double quotes and #
// literally, so their escapes come back off.
func escapeValue(value string) string {
	var out strings.Builder
	for _, run := range utils.ASCIIRuns(value) {
		if run[0] >= 0x80 {
			out.WriteString(run)
			continue
		}
		dumped := utils.RubyDump(run)
		dumped = dumped[1 : len(dumped)-1]
		dumped = strings.ReplaceAll(dumped, `\"`, `"`)
		out.WriteString(strings.ReplaceAll(dumped, `\#`, "#"))
	}
	return out.String()
}
