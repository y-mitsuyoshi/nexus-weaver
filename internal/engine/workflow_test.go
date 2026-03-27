package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkflow_Valid(t *testing.T) {
	yamlContent := `
name: "Test Pipeline"
steps:
  - id: "step1"
    type: "git_branch"
    branch_name: "feature/test"
  - id: "step2"
    type: "llm_task"
    agent_role: "PM"
    model: "gemini-cli"
    system_prompt_file: "./prompts/pm.txt"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/prd.md"
  - id: "step3"
    type: "loop"
    max_retries: 3
    command: "go test ./..."
    provider: "gemini-cli"
    fixer_prompt_file: "./prompts/fixer.txt"
    target_file: "./src/main.go"
  - id: "step4"
    type: "command_task"
    command: "echo done"
`
	tmpFile := writeTempYAML(t, yamlContent)

	wf, err := LoadWorkflow(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wf.Name != "Test Pipeline" {
		t.Errorf("expected name 'Test Pipeline', got %q", wf.Name)
	}
	if len(wf.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(wf.Steps))
	}

	// Step 1: git_branch
	s0 := wf.Steps[0]
	if s0.ID != "step1" || s0.Type != "git_branch" || s0.BranchName != "feature/test" {
		t.Errorf("step1 fields mismatch: %+v", s0)
	}

	// Step 2: llm_task
	s1 := wf.Steps[1]
	if s1.ID != "step2" || s1.Type != "llm_task" || s1.Model != "gemini-cli" {
		t.Errorf("step2 fields mismatch: %+v", s1)
	}
	if s1.AgentRole != "PM" {
		t.Errorf("expected agent_role 'PM', got %q", s1.AgentRole)
	}

	// Step 3: loop
	s2 := wf.Steps[2]
	if s2.ID != "step3" || s2.Type != "loop" || s2.MaxRetries != 3 {
		t.Errorf("step3 fields mismatch: %+v", s2)
	}

	// Step 4: command_task
	s3 := wf.Steps[3]
	if s3.ID != "step4" || s3.Type != "command_task" || s3.Command != "echo done" {
		t.Errorf("step4 fields mismatch: %+v", s3)
	}
}

func TestLoadWorkflow_MissingName(t *testing.T) {
	yamlContent := `
steps:
  - id: "step1"
    type: "llm_task"
    model: "gemini-cli"
`
	tmpFile := writeTempYAML(t, yamlContent)

	_, err := LoadWorkflow(tmpFile)
	if err == nil {
		t.Fatal("expected validation error for missing name, got nil")
	}
}

func TestLoadWorkflow_DuplicateID(t *testing.T) {
	yamlContent := `
name: "Dup"
steps:
  - id: "same"
    type: "command_task"
    command: "echo 1"
  - id: "same"
    type: "command_task"
    command: "echo 2"
`
	tmpFile := writeTempYAML(t, yamlContent)

	_, err := LoadWorkflow(tmpFile)
	if err == nil {
		t.Fatal("expected validation error for duplicate id, got nil")
	}
}

func TestLoadWorkflow_InvalidType(t *testing.T) {
	yamlContent := `
name: "Bad"
steps:
  - id: "step1"
    type: "unknown_type"
`
	tmpFile := writeTempYAML(t, yamlContent)

	_, err := LoadWorkflow(tmpFile)
	if err == nil {
		t.Fatal("expected validation error for unknown type, got nil")
	}
}

func TestLoadWorkflow_FileNotFound(t *testing.T) {
	_, err := LoadWorkflow("/nonexistent/path/workflow.yml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestResolveModelSpec(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		want     string
	}{
		{"ProviderOnly", "gemini-cli", "", "gemini-cli"},
		{"ProviderAndModel", "gemini-cli", "gemini-2.5-flash", "gemini-cli:gemini-2.5-flash"},
		{"ProviderWithDefault", "gemini-cli", "default", "gemini-cli"},
		{"CopilotWithModel", "copilot-cli", "claude-opus-4", "copilot-cli:claude-opus-4"},
		{"ModelOnly_BackwardCompat", "", "gemini-cli", "gemini-cli"},
		{"ModelOnly_WithColon", "", "gemini-cli:gemini-2.5-flash", "gemini-cli:gemini-2.5-flash"},
		{"BothEmpty", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveModelSpec(tc.provider, tc.model)
			if got != tc.want {
				t.Errorf("ResolveModelSpec(%q, %q) = %q, want %q", tc.provider, tc.model, got, tc.want)
			}
		})
	}
}

func TestLoadWorkflow_UnifiedProviderModel(t *testing.T) {
	yamlContent := `
name: "Unified Test"
steps:
  - id: "llm"
    type: "llm_task"
    provider: "gemini-cli"
    model: "gemini-2.5-flash"
    input_file: "./inbox/idea.txt"
  - id: "review"
    type: "review"
    provider: "copilot-cli"
    model: "claude-opus-4"
    target_file: "./docs/prd.md"
  - id: "loop"
    type: "loop"
    command: "go test ./..."
    max_retries: 3
    provider: "gemini-cli"
    target_file: "./cmd/main.go"
`
	tmpFile := writeTempYAML(t, yamlContent)

	wf, err := LoadWorkflow(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 全ステップが provider/model を共通フィールドとして持つこと
	if wf.Steps[0].Provider != "gemini-cli" || wf.Steps[0].Model != "gemini-2.5-flash" {
		t.Errorf("llm step: got provider=%q model=%q", wf.Steps[0].Provider, wf.Steps[0].Model)
	}
	if wf.Steps[1].Provider != "copilot-cli" || wf.Steps[1].Model != "claude-opus-4" {
		t.Errorf("review step: got provider=%q model=%q", wf.Steps[1].Provider, wf.Steps[1].Model)
	}
	if wf.Steps[2].Provider != "gemini-cli" {
		t.Errorf("loop step: got provider=%q", wf.Steps[2].Provider)
	}
}
