package control

import (
	"fmt"
	"time"
)

// Decision is the outcome of a policy evaluation.
type Decision struct {
	Action string // "allow" | "deny" | "warn"
	Reason string
	Detail string
}

// Engine evaluates policy decisions against live budget state.
type Engine struct {
	policy  Policy
	budgets *BudgetStore
	denials *DenialLog
}

// NewEngine creates an Engine with the given policy and stores.
func NewEngine(policy Policy, budgets *BudgetStore, denials *DenialLog) *Engine {
	return &Engine{
		policy:  policy,
		budgets: budgets,
		denials: denials,
	}
}

// PreToolUse evaluates policy before a tool executes.
// Order: denylist → allowlist → session budget → daily budget → warn fraction → allow.
func (e *Engine) PreToolUse(sessionID, tool, inputSummary string) Decision {
	target := tool
	if inputSummary != "" {
		target = tool + ":" + inputSummary
	}

	// 1. Denylist check.
	if Match(e.policy.Tools.Denylist, target) {
		d := Decision{
			Action: "deny",
			Reason: "tool_denylisted",
			Detail: fmt.Sprintf("tool %q matched denylist", target),
		}
		_ = e.denials.Append(Denial{
			Timestamp: time.Now().UTC(),
			SessionID: sessionID,
			Tool:      target,
			Reason:    d.Reason,
			Detail:    d.Detail,
		})
		return d
	}

	// 2. Allowlist check.
	if len(e.policy.Tools.Allowlist) > 0 && !Match(e.policy.Tools.Allowlist, target) {
		d := Decision{
			Action: "deny",
			Reason: "tool_not_allowlisted",
			Detail: fmt.Sprintf("tool %q not in allowlist", target),
		}
		_ = e.denials.Append(Denial{
			Timestamp: time.Now().UTC(),
			SessionID: sessionID,
			Tool:      target,
			Reason:    d.Reason,
			Detail:    d.Detail,
		})
		return d
	}

	// 3. Session budget check.
	if e.policy.Budget.SessionMaxUSD > 0 {
		b, err := e.budgets.Get(sessionID)
		if err == nil && b.CostUSD >= e.policy.Budget.SessionMaxUSD {
			d := Decision{
				Action: "deny",
				Reason: "budget_exceeded",
				Detail: fmt.Sprintf("session cost $%.4f >= cap $%.2f", b.CostUSD, e.policy.Budget.SessionMaxUSD),
			}
			_ = e.denials.Append(Denial{
				Timestamp: time.Now().UTC(),
				SessionID: sessionID,
				Tool:      target,
				Reason:    d.Reason,
				Detail:    d.Detail,
			})
			return d
		}
	}

	// 4. Daily budget check.
	if e.policy.Budget.DailyMaxUSD > 0 {
		daily, err := e.budgets.DailyTotal()
		if err == nil && daily >= e.policy.Budget.DailyMaxUSD {
			d := Decision{
				Action: "deny",
				Reason: "budget_exceeded",
				Detail: fmt.Sprintf("daily total $%.4f >= cap $%.2f", daily, e.policy.Budget.DailyMaxUSD),
			}
			_ = e.denials.Append(Denial{
				Timestamp: time.Now().UTC(),
				SessionID: sessionID,
				Tool:      target,
				Reason:    d.Reason,
				Detail:    d.Detail,
			})
			return d
		}
	}

	// 5. Warn fraction check.
	if e.policy.Budget.SessionMaxUSD > 0 && e.policy.Budget.WarnAtFraction > 0 {
		b, err := e.budgets.Get(sessionID)
		if err == nil && b.CostUSD >= e.policy.Budget.WarnAtFraction*e.policy.Budget.SessionMaxUSD {
			return Decision{
				Action: "warn",
				Reason: "approaching_budget",
				Detail: fmt.Sprintf("session cost $%.4f >= %.0f%% of cap $%.2f", b.CostUSD, e.policy.Budget.WarnAtFraction*100, e.policy.Budget.SessionMaxUSD),
			}
		}
	}

	return Decision{Action: "allow"}
}

// PostToolUse updates the session budget with token usage from the assistant turn.
func (e *Engine) PostToolUse(sessionID, model string, inputTokens, outputTokens int) error {
	_, err := e.budgets.Add(sessionID, model, inputTokens, outputTokens, e.policy.Models.Pricing)
	return err
}
