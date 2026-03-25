package engine

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Step はワークフロー内の個々の実行ステップを定義します。
// ステップのタイプ（llm_task / loop / command_task / git_branch / review / git_push）に応じて異なるフィールドが使用されます。
type Step struct {
	// 共通フィールド
	ID   string `yaml:"id"`
	Type string `yaml:"type"` // "llm_task", "loop", "command_task", "git_branch", "review", "git_push"

	// llm_task 用フィールド
	AgentRole        string `yaml:"agent_role,omitempty"`
	Model            string `yaml:"model,omitempty"`
	SystemPromptFile string `yaml:"system_prompt_file,omitempty"`
	InputFile        string `yaml:"input_file,omitempty"`
	OutputFile       string `yaml:"output_file,omitempty"`

	// loop 用フィールド
	MaxRetries      int    `yaml:"max_retries,omitempty"`
	Command         string `yaml:"command,omitempty"`
	FixerModel      string `yaml:"fixer_model,omitempty"`
	FixerPromptFile string `yaml:"fixer_prompt_file,omitempty"`
	TargetFile      string `yaml:"target_file,omitempty"`

	// git_branch 用フィールド
	BranchName     string `yaml:"branch_name,omitempty"`
	BranchNameFile string `yaml:"branch_name_file,omitempty"`

	// review 用フィールド
	ReviewerModel    string `yaml:"reviewer_model,omitempty"`
	ReviewPromptFile string `yaml:"review_prompt_file,omitempty"`

	// git_push 用フィールド
	Remote string `yaml:"remote,omitempty"`
}

// Workflow はYAMLで定義された一連の開発パイプラインを管理する構造体です。
type Workflow struct {
	Name  string `yaml:"name"`
	Steps []Step `yaml:"steps"`
}

// LoadWorkflow は指定されたパスのYAMLファイルを読み込み、
// Workflow 構造体にマッピングして返します。
func LoadWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file %s: %w", path, err)
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	if err := validateWorkflow(&wf); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

	return &wf, nil
}

// validateWorkflow はワークフロー定義の基本的なバリデーションを行います。
func validateWorkflow(wf *Workflow) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(wf.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	seenIDs := make(map[string]bool)
	for i, step := range wf.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d: id is required", i)
		}
		if seenIDs[step.ID] {
			return fmt.Errorf("step %d: duplicate id %q", i, step.ID)
		}
		seenIDs[step.ID] = true

		switch step.Type {
		case "llm_task":
			if step.Model == "" {
				return fmt.Errorf("step %q: model is required for llm_task", step.ID)
			}
		case "loop":
			if step.Command == "" {
				return fmt.Errorf("step %q: command is required for loop", step.ID)
			}
			if step.MaxRetries <= 0 {
				return fmt.Errorf("step %q: max_retries must be > 0 for loop", step.ID)
			}
			if step.TargetFile == "" {
				return fmt.Errorf("step %q: target_file is required for loop to apply fixes", step.ID)
			}
		case "command_task":
			if step.Command == "" {
				return fmt.Errorf("step %q: command is required for command_task", step.ID)
			}
		case "git_branch":
			if step.BranchName == "" && step.BranchNameFile == "" {
				return fmt.Errorf("step %q: branch_name or branch_name_file is required for git_branch", step.ID)
			}
		case "review":
			if step.ReviewerModel == "" {
				return fmt.Errorf("step %q: reviewer_model is required for review", step.ID)
			}
			if step.TargetFile == "" {
				return fmt.Errorf("step %q: target_file is required for review", step.ID)
			}
		case "git_push":
			// remote is optional, defaults to origin
		default:
			return fmt.Errorf("step %q: unknown type %q", step.ID, step.Type)
		}
	}
	return nil
}
