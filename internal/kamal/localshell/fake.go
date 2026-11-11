package localshell

import "fmt"

// Fake is a Shell for tests. It returns the Result stored for each command and
// records every command it receives. An unknown command is an error, so a test
// notices a command it did not expect.
//
//	shell := &localshell.Fake{Results: map[string]localshell.Result{"git rev-parse HEAD": {Stdout: "abc\n", Success: true}}}
type Fake struct {
	Results map[string]Result
	Calls   []string
}

// Run returns the stored Result for command.
func (f *Fake) Run(command string) (Result, error) {
	f.Calls = append(f.Calls, command)
	result, ok := f.Results[command]
	if !ok {
		return Result{}, fmt.Errorf("localshell.Fake: unexpected command %q: want one of the keys of Results", command)
	}
	return result, nil
}
