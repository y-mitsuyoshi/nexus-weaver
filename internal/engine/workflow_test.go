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
    type: "llm_task"
    agent_role: "PM"
    model: "gemini-cli"
    system_prompt_file: "./prompts/pm.txt"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/prd.md"
  - id: "step2"
    type: "loop"
    max_retries: 3
    command: "go test ./..."
    fixer_model: "gemini-cli"
    fixer_prompt_file: "./prompts/fixer.txt"
  - id: "step3"
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
	if len(wf.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(wf.Steps))
	}

	// Step 1: llm_task
	s1 := wf.Steps[0]
	if s1.ID != "step1" || s1.Type != "llm_task" || s1.Model != "gemini-cli" {
		t.Errorf("step1 fields mismatch: %+v", s1)
	}
	if s1.AgentRole != "PM" {
		t.Errorf("expected agent_role 'PM', got %q", s1.AgentRole)
	}

	// Step 2: loop
	s2 := wf.Steps[1]
	if s2.ID != "step2" || s2.Type != "loop" || s2.MaxRetries != 3 {
		t.Errorf("step2 fields mismatch: %+v", s2)
	}

	// Step 3: command_task
	s3 := wf.Steps[2]
	if s3.ID != "step3" || s3.Type != "command_task" || s3.Command != "echo done" {
		t.Errorf("step3 fields mismatch: %+v", s3)
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
