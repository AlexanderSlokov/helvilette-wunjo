package configuration

// Ports lib/kamal/configuration/boot.rb at Kamal 2.12.0.

import (
	"strconv"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/yamlord"
)

// Boot controls how many hosts start a new release at once.
type Boot struct {
	config    *yamlord.Ordered
	hostCount int
}

func newBoot(config *yamlord.Value, hostCount int) (*Boot, error) {
	if config.IsNull() {
		config = yamlord.NewMap(nil)
	}
	if err := (validator{context: "boot"}).againstExample(config, example("boot")); err != nil {
		return nil, err
	}
	return &Boot{config: config.Map(), hostCount: hostCount}, nil
}

// Limit returns the number of hosts per batch, or nil when unset. A percentage is
// of all hosts, at least 1.
func (b *Boot) Limit() *yamlord.Value {
	limit := b.config.At("limit")
	if !strings.HasSuffix(limit.ToS(), "%") {
		return limit
	}
	percent := leadingInt(limit.ToS())
	return yamlord.NewInt(int64(max(b.hostCount*percent/100, 1)))
}

// leadingInt follows Ruby's String#to_i: the leading digits, or 0.
func leadingInt(text string) int {
	text = strings.TrimSpace(text)
	end := 0
	for end < len(text) && (text[end] >= '0' && text[end] <= '9' || end == 0 && (text[end] == '-' || text[end] == '+')) {
		end++
	}
	n, _ := strconv.Atoi(text[:end])
	return n
}

// Wait returns the seconds to wait between batches, or nil.
func (b *Boot) Wait() *yamlord.Value { return b.config.At("wait") }

// ParallelRoles returns whether roles boot in parallel, or nil.
func (b *Boot) ParallelRoles() *yamlord.Value { return b.config.At("parallel_roles") }
