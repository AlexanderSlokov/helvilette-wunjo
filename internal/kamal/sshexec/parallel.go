package sshexec

// Ports SSHKit::Runner::Parallel with Kamal's CompleteAll patch, and the group
// runner, from SSHKit 1.25.1 and lib/kamal/sshkit_with_ext.rb at Kamal 2.12.0.

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ExecuteError wraps the failure of one host, as SSHKit::Runner::ExecuteError.
type ExecuteError struct {
	Message string
	Cause   error
}

func (e *ExecuteError) Error() string { return e.Message }

func (e *ExecuteError) Unwrap() error { return e.Cause }

// Runner opens a session per host and runs a block on each.
type Runner struct {
	Backend Backend
	Out     *Formatter
	NewID   func() string
	Sleep   func(time.Duration)
}

// On runs fn on every host in parallel and waits for all of them, then reports
// every failure, as Kamal's patched parallel runner does.
//
//	err := runner.On([]string{"1.1.1.1"}, func(s *sshexec.Session) error { return s.Execute(cmd) })
func (r *Runner) On(hosts []string, fn func(*Session) error) error {
	errs := make([]error, len(hosts))
	var wg sync.WaitGroup
	for i, host := range hosts {
		wg.Go(func() { errs[i] = r.onHost(host, fn) })
	}
	wg.Wait()
	return combineErrors(errs, "hosts")
}

// InGroups runs fn on hosts in groups of limit, one group after another, waiting
// wait between groups, as SSHKit's group runner does for boot limits.
func (r *Runner) InGroups(hosts []string, limit int, wait time.Duration, fn func(*Session) error) error {
	for start := 0; start < len(hosts); start += limit {
		if start > 0 && wait > 0 {
			r.Sleep(wait)
		}
		if err := r.On(hosts[start:min(start+limit, len(hosts))], fn); err != nil {
			return err
		}
	}
	return nil
}

// Sequence runs fn on each host in turn, as SSHKit's sequence runner does.
func (r *Runner) Sequence(hosts []string, fn func(*Session) error) error {
	for _, host := range hosts {
		if err := r.onHost(host, fn); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) onHost(host string, fn func(*Session) error) error {
	if err := fn(NewSession(host, r.Backend, r.Out, r.NewID)); err != nil {
		return &ExecuteError{Message: "Exception while executing on host " + host + ": " + err.Error(), Cause: err}
	}
	return nil
}

// combineErrors returns nil, the one error, or the first error with every message,
// as "Exceptions on 2 hosts:\n...".
func combineErrors(errs []error, noun string) error {
	var failed []error
	for _, err := range errs {
		if err != nil {
			failed = append(failed, err)
		}
	}
	switch len(failed) {
	case 0:
		return nil
	case 1:
		return failed[0]
	}
	messages := []string{fmt.Sprintf("Exceptions on %d %s:", len(failed), noun)}
	for _, err := range failed {
		messages = append(messages, err.Error())
	}
	return &ExecuteError{Message: strings.Join(messages, "\n"), Cause: failed[0]}
}

// RoleHosts is one role and its hosts, for OnRoles.
type RoleHosts struct {
	Role  string
	Hosts []string
}

// OnRoles runs fn per role. In parallel, each role runs in its own goroutine, so a
// host in two roles gets two sessions. Otherwise hosts run in parallel and each host
// runs its roles in order, as Kamal's on_roles does.
func (r *Runner) OnRoles(roles []RoleHosts, hosts []string, parallel bool, fn func(*Session, string) error) error {
	if !parallel {
		return r.On(hosts, func(s *Session) error { return eachRoleOn(s, roles, fn) })
	}
	errs := make([]error, len(roles))
	var wg sync.WaitGroup
	for i, role := range roles {
		roleHosts := intersect(role.Hosts, hosts)
		if len(roleHosts) == 0 {
			continue
		}
		wg.Go(func() { errs[i] = r.onRole(role.Role, roleHosts, fn) })
	}
	wg.Wait()
	return combineErrors(errs, "roles")
}

func (r *Runner) onRole(role string, hosts []string, fn func(*Session, string) error) error {
	err := r.On(hosts, func(s *Session) error { return fn(s, role) })
	if err != nil {
		return &ExecuteError{Message: "Exception while executing on " + role + ": " + err.Error(), Cause: err}
	}
	return nil
}

func eachRoleOn(s *Session, roles []RoleHosts, fn func(*Session, string) error) error {
	for _, role := range roles {
		if contains(role.Hosts, s.Host) {
			if err := fn(s, role.Role); err != nil {
				return err
			}
		}
	}
	return nil
}

func intersect(a, b []string) []string {
	var out []string
	for _, item := range a {
		if contains(b, item) {
			out = append(out, item)
		}
	}
	return out
}

func contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}
