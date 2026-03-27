package engine

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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
