package engine

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/y-mitsuyoshi/nexus-weaver/internal/fs"
	"github.com/y-mitsuyoshi/nexus-weaver/internal/llm"
)

// Engine はワークフローの各ステップを順番に実行するステートマシンです。
type Engine struct {
	Logger *slog.Logger
}

// NewEngine は構造化ログ付きの新しい Engine インスタンスを返します。
func NewEngine(logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{Logger: logger}
}

// Run はワークフローの全ステップを順番に実行します。
// 各ステップは Type に応じて異なるロジックで処理されます。
func (e *Engine) Run(wf *Workflow) error {
	e.Logger.Info("Starting workflow", "name", wf.Name, "steps", len(wf.Steps))

	// 現在のブランチを確認（保護ブランチでの直接実行を防止）
	currentBranch, err := fs.GetCurrentBranch()
	if err != nil {
		e.Logger.Warn("Could not determine current git branch", "error", err)
	} else if fs.IsProtectedBranch(currentBranch) {
		// 保護ブランチの場合、ワークフローの最初のステップが git_branch でない限り実行を拒否
		if len(wf.Steps) > 0 && wf.Steps[0].Type != "git_branch" {
			return fmt.Errorf("direct execution on protected branch %q is not allowed. Please use 'git_branch' step or switch to a feature branch", currentBranch)
		}
	}

	for i, step := range wf.Steps {
		e.Logger.Info("Executing step",
			"index", i+1,
			"total", len(wf.Steps),
			"id", step.ID,
			"type", step.Type,
		)

		var err error
		switch step.Type {
		case "llm_task":
			err = e.runLLMTask(step)
		case "loop":
			err = e.runTestLoop(step)
		case "command_task":
			err = e.runCommandTask(step)
		case "git_branch":
			err = e.runGitBranch(step)
		case "review":
			err = e.runReviewTask(step)
		case "git_push":
			err = e.runGitPush(step)
		default:
			err = fmt.Errorf("unknown step type: %s", step.Type)
		}

		if err != nil {
			e.Logger.Error("Step failed", "id", step.ID, "error", err)
			return fmt.Errorf("step %q failed: %w", step.ID, err)
		}

		e.Logger.Info("Step completed", "id", step.ID)
	}

	e.Logger.Info("Workflow completed successfully", "name", wf.Name)
	return nil
}

// runLLMTask は llm_task タイプのステップを実行します。
// 1. 指定モデルのプロバイダを取得
// 2. システムプロンプトと入力ファイルを読み込み
// 3. LLM に Generate を依頼
// 4. 結果を出力ファイルに書き込み
func (e *Engine) runLLMTask(step Step) error {
	provider, err := llm.GetProvider(step.Model)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// システムプロンプトの読み込み
	systemPrompt := ""
	if step.SystemPromptFile != "" {
		systemPrompt, err = fs.ReadFile(step.SystemPromptFile)
		if err != nil {
			return fmt.Errorf("failed to read system prompt: %w", err)
		}
	}

	// 入力ファイルの読み込み
	inputData := ""
	if step.InputFile != "" {
		inputData, err = fs.ReadFile(step.InputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
	}

	e.Logger.Info("Generating with LLM",
		"model", step.Model,
		"role", step.AgentRole,
	)

	result, err := provider.Generate(systemPrompt, inputData)
	if err != nil {
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	if step.OutputFile != "" {
		outputData := result
		// Goなどのソースコードファイルの場合はコードブロックを抽出
		if strings.HasSuffix(step.OutputFile, ".go") || strings.HasSuffix(step.OutputFile, ".yml") {
			if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
				outputData = extracted
			} else if extracted, err := fs.ExtractCodeBlock(result, "go"); err == nil {
				outputData = extracted
			}
		}

		if err := fs.WriteFile(step.OutputFile, outputData); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		e.Logger.Info("Output written", "file", step.OutputFile)
	}

	return nil
}

// runTestLoop は loop タイプのステップを実行します。
// テストコマンドを実行し、失敗した場合は Fixer モデルに修正を依頼するループ処理です。
// 各リトライ前に Git 自動コミットを行い、暴走時のロールバックに備えます。
func (e *Engine) runTestLoop(step Step) error {
	preLoopHash := ""
	for i := 0; i < step.MaxRetries; i++ {
		e.Logger.Info("Test loop iteration",
			"attempt", i+1,
			"max", step.MaxRetries,
		)

		// 安全装置: テスト実行前に自動コミット（コミット前の HEAD を返す）
		if baseHash, err := fs.AutoCommit(fmt.Sprintf("pre-test: attempt %d/%d for %s", i+1, step.MaxRetries, step.ID)); err != nil {
			e.Logger.Warn("Auto-commit failed (continuing)", "error", err)
		} else if preLoopHash == "" {
			preLoopHash = baseHash
		}

		// テストコマンド実行
		testErr := runShellCommand(step.Command)
		if testErr == nil {
			e.Logger.Info("Tests passed!")
			return nil
		}

		e.Logger.Warn("Tests failed", "error", testErr, "attempt", i+1)

		// 最後のリトライでも失敗した場合はロールバックしてループ終了
		if i == step.MaxRetries-1 {
			e.Logger.Error("Test loop failed, rolling back to pre-test state")
			if preLoopHash == "" {
				e.Logger.Warn("No pre-test commit recorded; skipping rollback")
			} else {
				if rbErr := fs.Rollback(preLoopHash); rbErr != nil {
					e.Logger.Error("Rollback failed", "error", rbErr)
				}
			}
			return fmt.Errorf("test loop exhausted after %d retries: %w", step.MaxRetries, testErr)
		}

		// Fixer モデルに修正を依頼
		if err := e.runFixer(step, testErr.Error()); err != nil {
			return fmt.Errorf("fixer failed: %w", err)
		}
	}
	return nil
}

// runFixer はテスト失敗時にエラーログを Fixer モデルに渡して修正を依頼します。
func (e *Engine) runFixer(step Step, errOutput string) error {
	if step.FixerModel == "" {
		return fmt.Errorf("no fixer_model specified for loop step %q", step.ID)
	}

	provider, err := llm.GetProvider(step.FixerModel)
	if err != nil {
		return fmt.Errorf("failed to get fixer provider: %w", err)
	}

	fixerPrompt := ""
	if step.FixerPromptFile != "" {
		fixerPrompt, err = fs.ReadFile(step.FixerPromptFile)
		if err != nil {
			return fmt.Errorf("failed to read fixer prompt: %w", err)
		}
	}

	// エラー出力を含むユーザープロンプトを構築
	userPrompt := fmt.Sprintf("以下のテストが失敗しました。エラーを修正してください。\n\nコマンド: %s\n\nエラー出力:\n%s",
		step.Command, errOutput)

	e.Logger.Info("Running fixer", "model", step.FixerModel)

	result, err := provider.Generate(fixerPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("fixer generation failed: %w", err)
	}

	e.Logger.Info("Fixer response received", "length", len(result))

	// result からコードブロックを抽出して対象ファイルに適用する
	fixCode := result
	// 言語指定なし、または "go" 指定のブロックを探す
	if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
		fixCode = extracted
	} else if extracted, err := fs.ExtractCodeBlock(result, "go"); err == nil {
		fixCode = extracted
	} else {
		e.Logger.Warn("No code block found in fixer response, using full response as fix")
	}

	if err := fs.WriteFile(step.TargetFile, fixCode); err != nil {
		return fmt.Errorf("failed to apply fix to %s: %w", step.TargetFile, err)
	}

	e.Logger.Info("Fix applied successfully", "file", step.TargetFile)
	return nil
}

// runCommandTask は command_task タイプのステップを実行します。
// PR作成などのシステムコマンドを os/exec で実行します。
func (e *Engine) runCommandTask(step Step) error {
	e.Logger.Info("Running command", "command", step.Command)
	return runShellCommand(step.Command)
}

// runGitBranch は git_branch タイプのステップを実行します。
func (e *Engine) runGitBranch(step Step) error {
	branchName := step.BranchName
	if branchName == "" && step.BranchNameFile != "" {
		content, err := fs.ReadFile(step.BranchNameFile)
		if err != nil {
			return fmt.Errorf("failed to read branch name file %s: %w", step.BranchNameFile, err)
		}
		branchName = strings.TrimSpace(content)
	}

	if branchName == "" {
		return fmt.Errorf("branch name is empty")
	}

	// プレフィックスの強制（既に含まれている場合は重複させない）
	if !strings.HasPrefix(branchName, "feature/") &&
		!strings.HasPrefix(branchName, "fix/") &&
		!strings.HasPrefix(branchName, "improvement/") {
		branchName = "feature/" + branchName
	}

	e.Logger.Info("Creating new branch", "name", branchName)
	if err := fs.CreateBranch(branchName); err != nil {
		return fmt.Errorf("failed to create branch %q: %w", branchName, err)
	}
	return nil
}

// runReviewTask は review タイプのステップを実行し、ユーザーの承認を求めます。
func (e *Engine) runReviewTask(step Step) error {
	provider, err := llm.GetProvider(step.ReviewerModel)
	if err != nil {
		return fmt.Errorf("failed to get reviewer: %w", err)
	}

	reviewPrompt := ""
	if step.ReviewPromptFile != "" {
		reviewPrompt, err = fs.ReadFile(step.ReviewPromptFile)
		if err != nil {
			return fmt.Errorf("failed to read review prompt: %w", err)
		}
	}

	targetContent, err := fs.ReadFile(step.TargetFile)
	if err != nil {
		return fmt.Errorf("failed to read target file for review: %w", err)
	}

	userPrompt := fmt.Sprintf("以下のファイルをレビューしてください:\n\nファイル: %s\n\n内容:\n%s",
		step.TargetFile, targetContent)

	e.Logger.Info("Running reviewer", "model", step.ReviewerModel)
	result, err := provider.Generate(reviewPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("review generation failed: %w", err)
	}

	// レビュー結果を表示
	fmt.Println("\n================================================================================")
	fmt.Println("🔍 コードレビュー結果")
	fmt.Println("================================================================================")
	fmt.Println(result)
	fmt.Println("================================================================================")

	// 履歴を保存
	reviewLogDir := "docs/reviews"
	reviewLogPath := fmt.Sprintf("%s/review-%s.md", reviewLogDir, step.ID)
	if err := fs.WriteFile(reviewLogPath, result); err != nil {
		e.Logger.Warn("Failed to save review log", "error", err)
	} else {
		e.Logger.Info("Review log saved", "file", reviewLogPath)
	}

	// 承認を求める
	for {
		fmt.Print("\nこの内容で Approve しますか？ [a]pprove / [r]equest changes / [q]uit: ")
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))

		switch input {
		case "a", "approve":
			e.Logger.Info("Review approved by user")
			return nil
		case "r", "request changes":
			return fmt.Errorf("review rejected by user (Request Changes)")
		case "q", "quit":
			os.Exit(0)
		default:
			fmt.Println("無効な入力です。'a', 'r', または 'q' を入力してください。")
		}
	}
}

// runGitPush は git_push タイプのステップを実行します。
func (e *Engine) runGitPush(step Step) error {
	remote := step.Remote
	if remote == "" {
		remote = "origin"
	}

	e.Logger.Info("Pushing to remote", "remote", remote)
	if err := fs.Push(remote); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	e.Logger.Info("Push completed successfully")
	return nil
}

// runShellCommand は sh -c でシェルコマンドを実行し、結合されたエラー出力を返します。
func runShellCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)

	var output bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		// エラーメッセージに stderr を含める
		errMsg := strings.TrimSpace(output.String())
		if errMsg != "" {
			return fmt.Errorf("%w\nstderr: %s", err, errMsg)
		}
		return err
	}
	return nil
}
