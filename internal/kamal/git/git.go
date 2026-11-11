// Package git reads the state of the local Git checkout that Kamal deploys from.
//
// Ports lib/kamal/git.rb at Kamal 2.12.0.
package git

import (
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

// Git runs git commands through a Shell.
//
//	g := git.Git{Shell: localshell.System{}}
//	revision, err := g.Revision()
type Git struct {
	Shell localshell.Shell
}

// Used reports whether the working directory is inside a Git repository.
func (g Git) Used() bool {
	result, err := g.Shell.Run("git rev-parse")
	return err == nil && result.Success
}

// UserName returns `git config user.name`.
func (g Git) UserName() (string, error) { return g.stripped("git config user.name") }

// Email returns `git config user.email`.
func (g Git) Email() (string, error) { return g.stripped("git config user.email") }

// Revision returns the commit hash of HEAD.
func (g Git) Revision() (string, error) { return g.stripped("git rev-parse HEAD") }

// UncommittedChanges returns `git status --porcelain`, empty for a clean tree.
func (g Git) UncommittedChanges() (string, error) { return g.stripped("git status --porcelain") }

// Root returns the top directory of the repository.
func (g Git) Root() (string, error) { return g.stripped("git rev-parse --show-toplevel") }

// UncommittedFiles returns the paths of modified files.
func (g Git) UncommittedFiles() ([]string, error) { return g.lines("git ls-files --modified") }

// UntrackedFiles returns the paths of untracked files, ignored ones included.
func (g Git) UntrackedFiles() ([]string, error) { return g.lines("git ls-files --others") }

func (g Git) stripped(command string) (string, error) {
	result, err := g.Shell.Run(command)
	return strings.TrimSpace(result.Stdout), err
}

func (g Git) lines(command string) ([]string, error) {
	result, err := g.Shell.Run(command)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.SplitAfter(result.Stdout, "\n") {
		if line != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines, nil
}
