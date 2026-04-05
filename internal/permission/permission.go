package permission

import (
	"github.com/ajistrying/nbcode2/internal/config"
	"github.com/ajistrying/nbcode2/internal/tools"
)

// Decision is the result of a permission check.
type Decision int

const (
	Allowed      Decision = iota // auto-approved
	NeedsApproval                // user must confirm
	Denied                       // blocked
)

// Checker evaluates whether a tool invocation is permitted.
type Checker struct {
	mode config.PermissionMode
}

// NewChecker creates a permission checker with the given mode.
func NewChecker(mode config.PermissionMode) *Checker {
	return &Checker{mode: mode}
}

// Check returns whether a tool execution should be allowed, needs approval, or is denied.
func (c *Checker) Check(t tools.Tool) Decision {
	switch c.mode {
	case config.PermissionYolo:
		return Allowed
	case config.PermissionAlwaysAsk:
		return NeedsApproval
	default: // risk-tiered
		switch t.RiskTier() {
		case tools.RiskSafe:
			return Allowed
		case tools.RiskMedium, tools.RiskHigh:
			return NeedsApproval
		default:
			return NeedsApproval
		}
	}
}
