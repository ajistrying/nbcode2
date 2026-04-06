package skills

import (
	"encoding/json"
	"fmt"

	"github.com/ajistrying/nbcode2/internal/tools"
)

// SkillTool implements tools.Tool, allowing the LLM to invoke skills.
type SkillTool struct {
	registry *Registry
	executor *Executor
}

// NewSkillTool creates the tool that the LLM uses to invoke skills.
func NewSkillTool(registry *Registry, executor *Executor) *SkillTool {
	return &SkillTool{
		registry: registry,
		executor: executor,
	}
}

type skillToolArgs struct {
	SkillName string `json:"skill_name"`
	Args      string `json:"args"`
}

func (t *SkillTool) Name() string { return "skill" }

func (t *SkillTool) Description() string {
	return "Execute a skill by name. Use this when a user's request matches an available skill from the skill catalog. Pass the skill name and any arguments."
}

func (t *SkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"skill_name": {
				"type": "string",
				"description": "The name of the skill to invoke"
			},
			"args": {
				"type": "string",
				"description": "Arguments to pass to the skill (optional)"
			}
		},
		"required": ["skill_name"]
	}`)
}

func (t *SkillTool) Execute(args json.RawMessage) (string, error) {
	var parsed skillToolArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return "", fmt.Errorf("invalid skill tool arguments: %w", err)
	}

	skill, err := t.registry.Get(parsed.SkillName)
	if err != nil {
		return "", fmt.Errorf("skill not found: %w", err)
	}

	if !skill.IsModelInvocable() {
		return "", fmt.Errorf("skill '%s' is not available for model invocation (disable-model-invocation: true)", parsed.SkillName)
	}

	result, err := t.executor.Execute(skill, parsed.Args)
	if err != nil {
		return "", fmt.Errorf("skill execution failed: %w", err)
	}

	return result, nil
}

func (t *SkillTool) RiskTier() tools.RiskTier {
	return tools.RiskMedium
}
