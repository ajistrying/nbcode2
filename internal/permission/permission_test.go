package permission

import (
	"encoding/json"
	"testing"

	"github.com/ajistrying/nbcode2/internal/config"
	"github.com/ajistrying/nbcode2/internal/tools"
)

// mockTool implements tools.Tool for testing permission checks.
type mockTool struct {
	name     string
	riskTier tools.RiskTier
}

func (m *mockTool) Name() string                            { return m.name }
func (m *mockTool) Description() string                     { return "mock" }
func (m *mockTool) Parameters() json.RawMessage             { return json.RawMessage(`{}`) }
func (m *mockTool) Execute(args json.RawMessage) (string, error) { return "", nil }
func (m *mockTool) RiskTier() tools.RiskTier                { return m.riskTier }

func TestRiskTieredMode(t *testing.T) {
	checker := NewChecker(config.PermissionRiskTiered)

	tests := []struct {
		name     string
		riskTier tools.RiskTier
		expected Decision
	}{
		{"safe tool", tools.RiskSafe, Allowed},
		{"medium tool", tools.RiskMedium, NeedsApproval},
		{"high tool", tools.RiskHigh, NeedsApproval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &mockTool{name: tt.name, riskTier: tt.riskTier}
			decision := checker.Check(tool)
			if decision != tt.expected {
				t.Errorf("expected decision %d, got %d", tt.expected, decision)
			}
		})
	}
}

func TestYoloMode(t *testing.T) {
	checker := NewChecker(config.PermissionYolo)

	tiers := []tools.RiskTier{tools.RiskSafe, tools.RiskMedium, tools.RiskHigh}
	for _, tier := range tiers {
		tool := &mockTool{name: "test", riskTier: tier}
		if decision := checker.Check(tool); decision != Allowed {
			t.Errorf("yolo mode should allow all tools, got %d for tier %d", decision, tier)
		}
	}
}

func TestAlwaysAskMode(t *testing.T) {
	checker := NewChecker(config.PermissionAlwaysAsk)

	tiers := []tools.RiskTier{tools.RiskSafe, tools.RiskMedium, tools.RiskHigh}
	for _, tier := range tiers {
		tool := &mockTool{name: "test", riskTier: tier}
		if decision := checker.Check(tool); decision != NeedsApproval {
			t.Errorf("always-ask mode should require approval, got %d for tier %d", decision, tier)
		}
	}
}

func TestRealToolTiers(t *testing.T) {
	// Verify that real tools have the expected risk tiers
	checker := NewChecker(config.PermissionRiskTiered)
	registry := tools.NewRegistry()

	safeTools := []string{"read_file", "list_files", "search"}
	for _, name := range safeTools {
		tool, _ := registry.Get(name)
		if checker.Check(tool) != Allowed {
			t.Errorf("expected %s to be auto-approved in risk-tiered mode", name)
		}
	}

	confirmTools := []string{"write_file", "edit_file", "bash", "web_search"}
	for _, name := range confirmTools {
		tool, _ := registry.Get(name)
		if checker.Check(tool) != NeedsApproval {
			t.Errorf("expected %s to need approval in risk-tiered mode", name)
		}
	}
}
