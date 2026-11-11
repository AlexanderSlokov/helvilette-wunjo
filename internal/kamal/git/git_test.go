package git

import (
	"reflect"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

func gitWith(command, stdout string) Git {
	return Git{Shell: &localshell.Fake{Results: map[string]localshell.Result{command: {Stdout: stdout, Success: true}}}}
}

func TestUserNameReadsUTF8(t *testing.T) {
	name, err := gitWith("git config user.name", "Сергей Федоров\n").UserName()
	if err != nil || name != "Сергей Федоров" {
		t.Fatalf("got %q, %v", name, err)
	}
}

func TestUncommittedChangesExist(t *testing.T) {
	if got, _ := gitWith("git status --porcelain", "M   file\n").UncommittedChanges(); got != "M   file" {
		t.Fatalf("got %q", got)
	}
}

func TestUncommittedChangesDoNotExist(t *testing.T) {
	if got, _ := gitWith("git status --porcelain", "").UncommittedChanges(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestUsedFollowsExitStatus(t *testing.T) {
	inside := Git{Shell: &localshell.Fake{Results: map[string]localshell.Result{"git rev-parse": {Success: true}}}}
	outside := Git{Shell: &localshell.Fake{Results: map[string]localshell.Result{"git rev-parse": {Success: false}}}}
	if !inside.Used() || outside.Used() {
		t.Fatal("Used did not follow the exit status")
	}
}

func TestUntrackedFiles(t *testing.T) {
	got, err := gitWith("git ls-files --others", "a.txt\n tmp/b.log\n").UntrackedFiles()
	if err != nil || !reflect.DeepEqual(got, []string{"a.txt", "tmp/b.log"}) {
		t.Fatalf("got %q, %v", got, err)
	}
}
