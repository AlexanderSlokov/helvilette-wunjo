package cli

// Ports the lock helpers of lib/kamal/cli/base.rb and lib/kamal/cli/lock.rb at
// Kamal 2.12.0.

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/sshexec"
)

const automaticDeployLockMessage = "Automatic deploy lock"

var (
	errLockHeld    = errors.New("lock held")
	errLockMissing = errors.New("lock missing")
)

var (
	cannotCreateDirectory = regexp.MustCompile(`cannot create directory`)
	noSuchFile            = regexp.MustCompile(`No such file or directory`)
)

// withLock runs fn holding the deploy lock, unless this run already holds it. A
// failure in fn releases the lock and reports the original error.
func (c *CLI) withLock(fn func() error) error {
	if c.kamal.HoldingLock {
		return fn()
	}
	if err := c.acquireLock(c.globalOptions()); err != nil {
		return err
	}
	if err := fn(); err != nil {
		if releaseErr := c.releaseLock(c.globalOptions()); releaseErr != nil {
			c.say("Error releasing the deploy lock: "+releaseErr.Error(), "red")
		}
		return err
	}
	return c.releaseLock(c.globalOptions())
}

func (c *CLI) acquireLock(opts Options) error {
	if err := c.ensureRunDirectory(opts); err != nil {
		return err
	}
	var err error
	if c.kamal.LockWait {
		err = c.acquireLockWithWait(opts)
	} else {
		err = c.raiseIfLocked(opts, func() error {
			c.say("Acquiring the deploy lock...", "magenta")
			return c.executeLockAcquire(opts, automaticDeployLockMessage)
		})
	}
	if err == nil {
		c.kamal.HoldingLock = true
	}
	return err
}

func (c *CLI) acquireLockWithWait(opts Options) error {
	timeout, interval := c.kamal.LockWaitTimeout, c.kamal.LockWaitInterval
	deadline := c.rt.Now().Add(time.Duration(timeout) * time.Second)
	c.say(fmt.Sprintf("Acquiring the deploy lock (waiting up to %ds)...", timeout), "magenta")
	detailsShown := false
	for {
		err := c.executeLockAcquire(opts, automaticDeployLockMessage)
		if !errors.Is(err, errLockHeld) {
			return err
		}
		if !detailsShown {
			if err := c.showLockHolder(opts); err != nil {
				return err
			}
			detailsShown = true
		}
		if err := c.waitForLock(deadline, timeout, interval); err != nil {
			return err
		}
	}
}

// showLockHolder prints the lock details and refuses to wait for a manual lock.
func (c *CLI) showLockHolder(opts Options) error {
	status, err := c.captureLockStatus(opts)
	if err != nil {
		return err
	}
	c.say("Deploy lock is held by:", "magenta")
	c.puts(status)
	if !strings.Contains(status, automaticDeployLockMessage) {
		return lockError("Deploy lock held manually, not waiting. Run 'kamal lock help' for more information")
	}
	return nil
}

func (c *CLI) waitForLock(deadline time.Time, timeout, interval int) error {
	remaining := int(deadline.Sub(c.rt.Now()).Seconds())
	if remaining <= 0 {
		c.say(fmt.Sprintf("Timed out after %ds waiting for the deploy lock", timeout), "red")
		return lockError("Timed out waiting for deploy lock")
	}
	c.say(fmt.Sprintf("Retrying in %ds (%ds remaining)...", interval, remaining), "magenta")
	c.rt.Sleep(time.Duration(min(interval, remaining)) * time.Second)
	return nil
}

func (c *CLI) releaseLock(opts Options) error {
	c.say("Releasing the deploy lock...", "magenta")
	if err := c.executeLockRelease(opts); err != nil {
		return err
	}
	c.kamal.HoldingLock = false
	return nil
}

// raiseIfLocked turns a held lock into a LockError that shows the lock details.
func (c *CLI) raiseIfLocked(opts Options, fn func() error) error {
	err := fn()
	if !errors.Is(err, errLockHeld) {
		return err
	}
	c.say("Deploy lock already in place!", "red")
	status, statusErr := c.captureLockStatus(opts)
	if statusErr != nil {
		return statusErr
	}
	c.puts(status)
	return lockError("Deploy lock found. Run 'kamal lock help' for more information")
}

func (c *CLI) executeLockAcquire(opts Options, message string) error {
	err := c.onPrimary(opts, func(s *sshexec.Session) error {
		return s.Execute(debugCmd(c.kamal.Lock().Acquire(message, c.config().Version())))
	})
	if err != nil && cannotCreateDirectory.MatchString(err.Error()) {
		return errLockHeld
	}
	return err
}

func (c *CLI) executeLockRelease(opts Options) error {
	err := c.onPrimary(opts, func(s *sshexec.Session) error { return s.Execute(debugCmd(c.kamal.Lock().Release())) })
	if err != nil && noSuchFile.MatchString(err.Error()) {
		return errLockMissing
	}
	return err
}

func (c *CLI) captureLockStatus(opts Options) (string, error) {
	var status string
	err := c.onPrimary(opts, func(s *sshexec.Session) error {
		var err error
		status, err = s.Capture(cmd(c.kamal.Lock().Status()))
		return err
	})
	if err != nil && noSuchFile.MatchString(err.Error()) {
		return "", errLockMissing
	}
	return status, err
}

func (c *CLI) onPrimary(opts Options, fn func(*sshexec.Session) error) error {
	return c.on(opts, []string{c.kamal.PrimaryHost()}, fn)
}

// lockStatus ports `kamal lock status`.
func (c *CLI) lockStatus(opts Options) error {
	return c.handleMissingLock(func() error {
		status, err := c.captureLockStatus(opts)
		if err == nil {
			c.puts(status)
		}
		return err
	})
}

// lockAcquire ports `kamal lock acquire`.
func (c *CLI) lockAcquire(opts Options) error {
	if err := c.ensureRunDirectory(opts); err != nil {
		return err
	}
	return c.raiseIfLocked(opts, func() error {
		if err := c.executeLockAcquire(opts, opts.String("message")); err != nil {
			return err
		}
		c.say("Acquired the deploy lock", "")
		return nil
	})
}

// lockRelease ports `kamal lock release`.
func (c *CLI) lockRelease(opts Options) error {
	return c.handleMissingLock(func() error {
		if err := c.executeLockRelease(opts); err != nil {
			return err
		}
		c.say("Released the deploy lock", "")
		return nil
	})
}

func (c *CLI) handleMissingLock(fn func() error) error {
	err := fn()
	if errors.Is(err, errLockMissing) {
		c.say("There is no deploy lock", "")
		return nil
	}
	return err
}
