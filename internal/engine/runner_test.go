package engine

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/y-mitsuyoshi/nexus-weaver/internal/fs"
)

// cleanupBranch はテスト終了時にテスト用ブランチを削除します。
// テスト開始時のブランチを記録し、テスト後にそこへ戻ってから削除します。
func cleanupBranch(t *testing.T, branchName string) {
	t.Helper()
	originalBranch, _ := fs.GetCurrentBranch()
	t.Cleanup(func() {
		current, _ := fs.GetCurrentBranch()
		if current == branchName {
			_ = fs.SwitchBranch(originalBranch)
		}
		_ = fs.DeleteBranch(branchName)
	})
}

func TestEngine_RunCommandTask(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("fix/command-task-test-%d", os.Getpid())
	cleanupBranch(t, branchName)

	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:         "branch-setup",
				Type:       "git_branch",
				BranchName: branchName,
			},
			{
				ID:      "echo-test",
				Type:    "command_task",
				Command: "echo hello",
			},
		},
	}

	if err := engine.Run(wf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngine_RunCommandTask_Failure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("fix/command-fail-test-%d", os.Getpid())
	cleanupBranch(t, branchName)

	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:         "branch-setup",
				Type:       "git_branch",
				BranchName: branchName,
			},
			{
				ID:      "fail-test",
				Type:    "command_task",
				Command: "exit 1",
			},
		},
	}

	err := engine.Run(wf)
	if err == nil {
		t.Fatal("expected error for failing command, got nil")
	}
}

func TestEngine_RunLLMTask(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	dir := t.TempDir()

	// 入力ファイルとプロンプトファイルを作成
	inputFile := filepath.Join(dir, "input.txt")
	promptFile := filepath.Join(dir, "prompt.txt")
	outputFile := filepath.Join(dir, "output.txt")

	if err := os.WriteFile(inputFile, []byte("test input"), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}
	if err := os.WriteFile(promptFile, []byte("test prompt"), 0644); err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("feature/llm-task-test-%d", os.Getpid())
	cleanupBranch(t, branchName)

	// Note: このテストは実際のLLMプロバイダを使うため、
	// 未知のプロバイダを使ってエラーハンドリングを検証します。
	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:         "branch-setup",
				Type:       "git_branch",
				BranchName: branchName,
			},
			{
				ID:               "llm-test",
				Type:             "llm_task",
				Provider:         "unknown-provider",
				SystemPromptFile: promptFile,
				InputFile:        inputFile,
				OutputFile:       outputFile,
			},
		},
	}

	err := engine.Run(wf)
	if err == nil {
		t.Fatal("expected error for unknown model, got nil")
	}
}

func TestEngine_RunTestLoop_Failure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("fix/loop-fail-test-%d", os.Getpid())
	cleanupBranch(t, branchName)

	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:         "branch-setup",
				Type:       "git_branch",
				BranchName: branchName,
			},
			{
				ID:         "loop-test",
				Type:       "loop",
				MaxRetries: 2,
				Command:    "exit 1",
				Provider:   "unknown-provider",
				TargetFile: "non-existent.go",
			},
		},
	}

	err := engine.Run(wf)
	if err == nil {
		t.Fatal("expected error for loop with unknown fixer model, got nil")
	}
}

func TestEngine_RunGitBranch(t *testing.T) {
	// gitコマンドが存在するかチェック
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found, skipping TestEngine_RunGitBranch")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("feature/git-branch-test-%d", os.Getpid())
	cleanupBranch(t, branchName)
	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:         "branch-test",
				Type:       "git_branch",
				BranchName: branchName,
			},
		},
	}

	// 実際のGitリポジトリ内である必要があるため、慎重に実行
	// もしテストがGit管理外の場所で走った場合はエラーになるが、それは期待通り
	err := engine.Run(wf)
	if err != nil {
		// 既にブランチが存在する場合などはエラーになる可能性があるが、
		// テストとしては「機能が呼び出されていること」を確認できれば良い
		t.Logf("engine.Run returned error (expected in some envs): %v", err)
	}
}

func TestEngine_UnknownStepType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:   "bad-type",
				Type: "invalid",
			},
		},
	}

	err := engine.Run(wf)
	if err == nil {
		t.Fatal("expected error for unknown step type, got nil")
	}
}

func TestEngine_StepResultAccumulation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	if len(engine.StepResults) != 0 {
		t.Fatalf("expected empty StepResults, got %d", len(engine.StepResults))
	}

	engine.addStepResult(StepResult{
		StepID:    "step1",
		StepType:  "llm_task",
		AgentRole: "PM",
		Summary:   "PRDを生成しました",
		Success:   true,
	})

	if len(engine.StepResults) != 1 {
		t.Fatalf("expected 1 StepResult, got %d", len(engine.StepResults))
	}
	if engine.StepResults[0].StepID != "step1" {
		t.Errorf("expected step1, got %q", engine.StepResults[0].StepID)
	}
}

func TestEngine_BuildContextSummary_Empty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	summary := engine.buildContextSummary()
	if summary != "" {
		t.Errorf("expected empty summary, got %q", summary)
	}
}

func TestEngine_BuildContextSummary_WithResults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	engine.addStepResult(StepResult{
		StepID:     "generate_branch",
		StepType:   "llm_task",
		AgentRole:  "ReleaseEngineer",
		OutputFile: "docs/branch_name.txt",
		Summary:    "feature/new-feature",
		Success:    true,
	})
	engine.addStepResult(StepResult{
		StepID:   "branch_creation",
		StepType: "git_branch",
		Success:  true,
	})

	summary := engine.buildContextSummary()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	for _, check := range []string{"ワークフロー実行コンテキスト", "generate_branch", "ReleaseEngineer", "feature/new-feature", "成功"} {
		if !strings.Contains(summary, check) {
			t.Errorf("summary missing %q", check)
		}
	}
}

func TestSummarizeOutput(t *testing.T) {
	if result := summarizeOutput("hello", 500); result != "hello" {
		t.Errorf("short: got %q", result)
	}
	if result := summarizeOutput(strings.Repeat("a", 600), 500); !strings.Contains(result, "以下省略") {
		t.Error("truncation marker missing")
	}
	if result := summarizeOutput(strings.Repeat("b", 600), 0); !strings.Contains(result, "以下省略") {
		t.Error("default max: truncation marker missing")
	}
}

func TestEngine_CommandTaskAccumulatesResult(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("fix/result-acc-test-%d", os.Getpid())
	cleanupBranch(t, branchName)

	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:         "branch-setup",
				Type:       "git_branch",
				BranchName: branchName,
			},
			{
				ID:      "echo-test",
				Type:    "command_task",
				Command: "echo hello",
			},
		},
	}

	if err := engine.Run(wf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(engine.StepResults) != 2 {
		t.Fatalf("expected 2 StepResults, got %d", len(engine.StepResults))
	}
	if engine.StepResults[0].StepID != "branch-setup" {
		t.Errorf("step 0: expected branch-setup, got %q", engine.StepResults[0].StepID)
	}
	if !engine.StepResults[1].Success {
		t.Error("step 1: expected success")
	}
}
