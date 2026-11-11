package sshexec

// Ports SSHKit::Backend::Netssh::DnsRetriable from lib/kamal/sshkit_with_ext.rb at
// Kamal 2.12.0.

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"
)

// dnsErrorMessage matches the messages of name resolution failures, as Kamal's
// DNS_ERROR_MESSAGE does. OpenSSH prints them on stderr.
var dnsErrorMessage = regexp.MustCompile(`(?i)getaddrinfo|Temporary failure in name resolution|Name or service not known|nodename nor servname provided|No address associated|failed to look up|resolve`)

const (
	dnsRetryBase   = 100 * time.Millisecond
	dnsRetryMax    = 2 * time.Second
	dnsRetryJitter = 100 * time.Millisecond
)

// DNSRetry retries a connection that failed on name resolution, with exponential
// backoff and jitter.
type DNSRetry struct {
	Retries int
	Sleep   func(time.Duration)
	Rand    func() float64
}

// Do runs connect, retrying up to Retries times while it fails with a DNS error.
//
//	err := retry.Do("example.com", out, func() error { return connect() })
func (d DNSRetry) Do(host string, out *Formatter, connect func() error) error {
	for attempt := 1; ; attempt++ {
		err := connect()
		if err == nil || !retryableDNSError(err) || attempt > d.Retries {
			return err
		}
		delay := d.delay(attempt)
		out.Log(Warn, fmt.Sprintf("Retrying DNS for %s (attempt %d/%d) in %0.2fs: %s", host, attempt, d.Retries, delay.Seconds(), err.Error()))
		d.Sleep(delay)
	}
}

func (d DNSRetry) delay(attempt int) time.Duration {
	backoff := time.Duration(math.Min(float64(dnsRetryBase)*math.Pow(2, float64(attempt-1)), float64(dnsRetryMax)))
	jitter := 0.0
	if d.Rand != nil {
		jitter = d.Rand()
	}
	return backoff + time.Duration(jitter*float64(dnsRetryJitter))
}

// DNSError reports a name resolution failure, as Resolv::ResolvError does.
type DNSError struct{ Message string }

func (e *DNSError) Error() string { return e.Message }

func retryableDNSError(err error) bool {
	var dns *DNSError
	if errors.As(err, &dns) {
		return true
	}
	var conn *ConnectionError
	return errors.As(err, &conn) && dnsErrorMessage.MatchString(conn.Message)
}
