package engine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// mockLLMProvider はテスト用のモック LLMProvider です。
type mockLLMProvider struct {
	Response string
	Err      error
}

func (m *mockLLMProvider) Generate(systemPrompt, userPrompt string) (string, error) {
	return m.Response, m.Err
}

func TestEngine_RunCommandTask(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
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

	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
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

	os.WriteFile(inputFile, []byte("test input"), 0644)
	os.WriteFile(promptFile, []byte("test prompt"), 0644)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	engine := NewEngine(logger)

	// Note: このテストは実際のLLMプロバイダを使うため、
	// 未知のモデル名を使ってエラーハンドリングを検証します。
	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:               "llm-test",
				Type:             "llm_task",
				Model:            "unknown-model",
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

	wf := &Workflow{
		Name: "Test",
		Steps: []Step{
			{
				ID:         "loop-test",
				Type:       "loop",
				MaxRetries: 2,
				Command:    "exit 1",
				FixerModel: "unknown-model",
				TargetFile: "non-existent.go",
			},
		},
	}

	err := engine.Run(wf)
	if err == nil {
		t.Fatal("expected error for loop with unknown fixer model, got nil")
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
