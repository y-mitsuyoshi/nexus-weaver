package engine

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo はテスト用の隔離された Git リポジトリを一時ディレクトリに作成し、
// カレントディレクトリをそこへ移動します。テスト終了時に自動で元のディレクトリへ戻ります。
// これにより実リポジトリの未コミット変更がテストで消えることを防ぎます。
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// git config
	for _, args := range [][]string{
		{"config", "user.email", "test@nexus-weaver.dev"},
		{"config", "user.name", "nexus-weaver-test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// 初期コミット（空リポジトリだと git checkout -b が動かないため）
	initFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(initFile, []byte("# test repo\n"), 0644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	// カレントディレクトリを一時リポジトリへ移動
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp repo: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	return dir
}

func TestEngine_RunCommandTask(t *testing.T) {
	setupTestRepo(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("fix/command-task-test-%d", os.Getpid())

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
	setupTestRepo(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("fix/command-fail-test-%d", os.Getpid())

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
	setupTestRepo(t)

	// テスト用の一時ファイルを作成（setupTestRepoのディレクトリ内）
	dir, _ := os.Getwd()
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
	setupTestRepo(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("fix/loop-fail-test-%d", os.Getpid())

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
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	setupTestRepo(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("feature/git-branch-test-%d", os.Getpid())

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

	if err := engine.Run(wf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Vars にブランチ名が登録されていること
	if engine.Vars["branch_name"] != branchName {
		t.Errorf("expected branch_name=%q, got %q", branchName, engine.Vars["branch_name"])
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
	setupTestRepo(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	branchName := fmt.Sprintf("fix/result-acc-test-%d", os.Getpid())

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
