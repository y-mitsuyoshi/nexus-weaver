package engine

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/y-mitsuyoshi/nexus-weaver/internal/fs"
	"github.com/y-mitsuyoshi/nexus-weaver/internal/llm"
)

// Engine はワークフローの各ステップを順番に実行するステートマシンです。
type Engine struct {
	Logger *slog.Logger
	// Vars はワークフロー実行中に動的に設定されるテンプレート変数です。
	// ステップのファイルパスや command 内の {{key}} が実行時に置換されます。
	Vars map[string]string
	// StepResults は完了したステップの結果を蓄積し、後続ステップにコンテキストを伝搬します。
	StepResults []StepResult
}

// NewEngine は構造化ログ付きの新しい Engine インスタンスを返します。
func NewEngine(logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{Logger: logger, Vars: make(map[string]string), StepResults: make([]StepResult, 0)}
}

// addStepResult はステップ実行結果を蓄積します。
func (e *Engine) addStepResult(result StepResult) {
	e.StepResults = append(e.StepResults, result)
}

// buildContextSummary は蓄積されたステップ結果からコンテキスト要約文字列を構築します。
func (e *Engine) buildContextSummary() string {
	if len(e.StepResults) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n---\n# ワークフロー実行コンテキスト（前ステップの成果）\n")
	sb.WriteString("以下は今回のワークフローで既に完了したステップの要約です。\n")
	sb.WriteString("これらの決定事項・成果物との一貫性を保ってください。\n\n")

	for _, r := range e.StepResults {
		status := "✅ 成功"
		if !r.Success {
			status = "❌ 失敗"
		}
		sb.WriteString(fmt.Sprintf("## ステップ: %s (%s) [%s]\n", r.StepID, r.AgentRole, status))
		if r.OutputFile != "" {
			sb.WriteString(fmt.Sprintf("出力先: %s\n", r.OutputFile))
		}
		if r.Summary != "" {
			sb.WriteString(fmt.Sprintf("要約:\n%s\n", r.Summary))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// summarizeOutput は出力テキストの先頭部分を要約として切り出します。
func summarizeOutput(content string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 500
	}
	content = strings.TrimSpace(content)
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "\n... (以下省略)"
}

// Run はワークフローの全ステップを順番に実行します。
// 連続する review ステップは「レビューゲート」としてグループ化され、
// いずれかの review で修正が発生した場合はグループ全体を最初からやり直します。
func (e *Engine) Run(wf *Workflow) error {
	e.Logger.Info("Starting workflow", "name", wf.Name, "steps", len(wf.Steps))

	// 現在のブランチを確認（保護ブランチでの直接実行を防止）
	currentBranch, err := fs.GetCurrentBranch()
	if err != nil {
		e.Logger.Warn("Could not determine current git branch", "error", err)
	} else if fs.IsProtectedBranch(currentBranch) {
		// 保護ブランチの場合、ワークフロー内に git_branch ステップが含まれていない限り実行を拒否
		hasGitBranch := false
		for _, s := range wf.Steps {
			if s.Type == "git_branch" {
				hasGitBranch = true
				break
			}
		}
		if !hasGitBranch {
			return fmt.Errorf("direct execution on protected branch %q is not allowed. Please use 'git_branch' step or switch to a feature branch", currentBranch)
		}
	}

	i := 0
	for i < len(wf.Steps) {
		step := wf.Steps[i]
		// テンプレート変数を解決
		e.resolveStepPaths(&step)
		wf.Steps[i] = step

		// 連続する review ステップをレビューゲートとしてグループ化
		if step.Type == "review" {
			reviewGroup := e.collectReviewGroup(wf.Steps, i)
			// レビューグループ内の各ステップも変数解決
			for j := range reviewGroup {
				e.resolveStepPaths(&reviewGroup[j])
			}
			e.Logger.Info("Review gate detected",
				"start_index", i+1,
				"count", len(reviewGroup),
				"ids", reviewGroupIDs(reviewGroup),
			)

			if err := e.runReviewGate(reviewGroup); err != nil {
				return err
			}

			i += len(reviewGroup)
			continue
		}

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
		case "git_push":
			err = e.runGitPush(step)
		default:
			err = fmt.Errorf("unknown step type: %s", step.Type)
		}

		if err != nil {
			e.Logger.Error("Step failed", "id", step.ID, "error", err)
			e.addStepResult(StepResult{
				StepID:    step.ID,
				StepType:  step.Type,
				AgentRole: step.AgentRole,
				Success:   false,
				Summary:   fmt.Sprintf("エラー: %s", err.Error()),
			})
			return fmt.Errorf("step %q failed: %w", step.ID, err)
		}

		// 成功したステップの結果を蓄積（LLM以外は最低限の記録）
		if step.Type != "llm_task" {
			e.addStepResult(StepResult{
				StepID:    step.ID,
				StepType:  step.Type,
				AgentRole: step.AgentRole,
				Success:   true,
			})
		}

		// ファイル出力があったステップの後は自動コミット（中間成果物の保全）
		if step.OutputFile != "" || step.Type == "command_task" {
			if _, acErr := fs.AutoCommit(fmt.Sprintf("auto: step %s completed", step.ID)); acErr != nil {
				e.Logger.Warn("Auto-commit after step failed (continuing)", "step", step.ID, "error", acErr)
			}
		}

		e.Logger.Info("Step completed", "id", step.ID)
		i++
	}

	// ワークフロー完了時に最終自動コミット（未コミットの成果物を確保）
	if _, err := fs.AutoCommit("auto: workflow completed"); err != nil {
		e.Logger.Warn("Final auto-commit failed", "error", err)
	}

	e.Logger.Info("Workflow completed successfully", "name", wf.Name)
	return nil
}

// collectReviewGroup はインデックス start から連続する review ステップを収集します。
func (e *Engine) collectReviewGroup(steps []Step, start int) []Step {
	var group []Step
	for j := start; j < len(steps) && steps[j].Type == "review"; j++ {
		group = append(group, steps[j])
	}
	return group
}

// reviewGroupIDs はレビューグループのステップID一覧を返します（ログ用）。
func reviewGroupIDs(group []Step) []string {
	ids := make([]string, len(group))
	for i, s := range group {
		ids[i] = s.ID
	}
	return ids
}

// runReviewGate は連続する review ステップ群を「レビューゲート」として実行します。
// いずれかの review で修正が発生した場合、全 review を最初からやり直します。
// 全 review が修正なしで approve されたらゲートを通過します。
func (e *Engine) runReviewGate(reviews []Step) error {
	// グループ全体のリトライ上限（個別の max_retries の合計を上限とする）
	totalMaxRetries := 0
	for _, r := range reviews {
		if r.MaxRetries > 0 {
			totalMaxRetries += r.MaxRetries
		} else {
			totalMaxRetries += 1
		}
	}

	preGateHash := ""
	for gateAttempt := 0; gateAttempt < totalMaxRetries; gateAttempt++ {
		e.Logger.Info("Review gate round",
			"round", gateAttempt+1,
			"max_rounds", totalMaxRetries,
		)

		// ゲート開始前の安全装置
		if baseHash, err := fs.AutoCommit(fmt.Sprintf("pre-review-gate: round %d", gateAttempt+1)); err != nil {
			e.Logger.Warn("Auto-commit failed (continuing)", "error", err)
		} else if preGateHash == "" {
			preGateHash = baseHash
		}

		fixApplied := false

		for _, reviewStep := range reviews {
			approved, fixed, err := e.runSingleReview(reviewStep)
			if err != nil {
				return fmt.Errorf("review step %q failed: %w", reviewStep.ID, err)
			}

			if fixed {
				fixApplied = true
			}

			if !approved {
				// ユーザーが reject した → ロールバックして終了
				if preGateHash != "" {
					e.Logger.Error("Review gate rejected, rolling back")
					if rbErr := fs.Rollback(preGateHash); rbErr != nil {
						e.Logger.Error("Rollback failed", "error", rbErr)
					}
				}
				return fmt.Errorf("review gate rejected at step %q", reviewStep.ID)
			}
		}

		if !fixApplied {
			// 全 review が修正なしで approve → ゲート通過！
			e.Logger.Info("✅ Review gate passed — all reviews approved without fixes")
			return nil
		}

		// 修正が発生した → 全 review をやり直し
		e.Logger.Warn("⚠️  Fix applied during review gate — restarting ALL reviews",
			"round", gateAttempt+1,
		)
	}

	// リトライ上限到達
	if preGateHash != "" {
		e.Logger.Error("Review gate exhausted, rolling back")
		if rbErr := fs.Rollback(preGateHash); rbErr != nil {
			e.Logger.Error("Rollback failed", "error", rbErr)
		}
	}
	return fmt.Errorf("review gate exhausted after %d rounds", totalMaxRetries)
}

// runLLMTask は llm_task タイプのステップを実行します。
// 1. 指定モデルのプロバイダを取得
// 2. システムプロンプトと入力ファイルを読み込み
// 3. コード生成時は既存プロジェクト構造を自動注入
// 4. LLM に Generate を依頼
// 5. 結果を出力ファイルに書き込み
func (e *Engine) runLLMTask(step Step) error {
	modelSpec := ResolveModelSpec(step.Provider, step.Model)
	provider, err := llm.GetProvider(modelSpec)
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

	// ワークフローコンテキスト（前ステップの成果要約）を注入
	contextSummary := e.buildContextSummary()
	if contextSummary != "" {
		inputData += contextSummary
	}

	// コード生成時は既存プロジェクト構造をコンテキストとして自動注入
	if isCodeFile(step.OutputFile) {
		ctx := collectCodebaseContext()
		inputData += ctx
		e.Logger.Debug("Auto-injected codebase context", "output_file", step.OutputFile)

		// 出力先ファイルが既に存在する場合はその内容も追加
		if existing, readErr := fs.ReadFile(step.OutputFile); readErr == nil {
			inputData += fmt.Sprintf("\n\n## 現在の %s の内容（互換性を維持してください）\n```\n%s\n```\n", step.OutputFile, existing)
		}
	}

	e.Logger.Info("Generating with LLM",
		"model", modelSpec,
		"role", step.AgentRole,
	)

	result, err := provider.Generate(systemPrompt, inputData)
	if err != nil {
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	// 出力バリデーション（ガードレール）: 空出力を拒否
	trimmed := strings.TrimSpace(result)
	if len(trimmed) == 0 {
		return fmt.Errorf("LLM returned empty output for step %q", step.ID)
	}
	if len(trimmed) < 10 && step.OutputFile != "" {
		e.Logger.Warn("LLM output suspiciously short",
			"step", step.ID,
			"length", len(trimmed),
			"content", trimmed,
		)
	}

	if step.OutputFile != "" {
		outputData := result
		// 出力ファイルの拡張子に応じて優先的に対応する言語のコードブロックを抽出する
		if strings.HasSuffix(step.OutputFile, ".go") {
			if extracted, err := fs.ExtractCodeBlock(result, "go"); err == nil {
				outputData = extracted
			} else if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
				outputData = extracted
			} else {
				return fmt.Errorf("failed to extract Go code block from LLM output for step %q (output may not contain valid code fences)", step.ID)
			}
		} else if strings.HasSuffix(step.OutputFile, ".yml") || strings.HasSuffix(step.OutputFile, ".yaml") {
			if extracted, err := fs.ExtractCodeBlock(result, "yaml"); err == nil {
				outputData = extracted
			} else if extracted, err := fs.ExtractCodeBlock(result, "yml"); err == nil {
				outputData = extracted
			} else if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
				outputData = extracted
			}
		} else {
			if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
				outputData = extracted
			}
		}

		if err := fs.WriteFile(step.OutputFile, outputData); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		e.Logger.Info("Output written", "file", step.OutputFile)
	}

	// ステップ結果を蓄積（後続ステップへのコンテキスト伝搬用）
	e.addStepResult(StepResult{
		StepID:     step.ID,
		StepType:   step.Type,
		AgentRole:  step.AgentRole,
		OutputFile: step.OutputFile,
		Summary:    summarizeOutput(result, 500),
		Success:    true,
	})

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
			e.Logger.Error("Fixer failed", "attempt", i+1, "error", err)
			if preLoopHash != "" {
				e.Logger.Info("Rolling back due to fixer failure")
				if rbErr := fs.Rollback(preLoopHash); rbErr != nil {
					e.Logger.Error("Rollback failed", "error", rbErr)
				}
			}
			return fmt.Errorf("fixer failed: %w", err)
		}
	}
	return nil
}

// runFixer はテスト失敗時またはレビュー指摘時に Fixer モデルに修正を依頼します。
// ステップの provider/model をそのまま使用します。
func (e *Engine) runFixer(step Step, errorOrReviewOutput string) error {
	modelSpec := ResolveModelSpec(step.Provider, step.Model)
	if modelSpec == "" {
		return fmt.Errorf("no provider/model specified for step %q", step.ID)
	}

	provider, err := llm.GetProvider(modelSpec)
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

	// 修正対象ファイルの現在の内容を読み込む
	currentContent := ""
	if step.TargetFile != "" {
		if content, readErr := fs.ReadFile(step.TargetFile); readErr == nil {
			currentContent = content
		}
	}

	// プロンプトの構築をコンテキスト（テスト失敗かレビューか）に合わせる
	contextDesc := "以下のテストが失敗しました。エラーを修正してください。\n\nコマンド: " + step.Command
	if step.Type == "review" {
		contextDesc = "以下のレビュー指摘を受けました。コードを修正してください。"
	}

	userPrompt := fmt.Sprintf("%s\n\n指摘内容・エラー出力:\n%s",
		contextDesc, errorOrReviewOutput)

	// ワークフローコンテキスト（前ステップの成果要約）を注入して修正の方向性を伝える
	fixerContext := e.buildContextSummary()
	if fixerContext != "" {
		userPrompt += fixerContext
	}

	// 修正対象ファイルの現在の内容を含める
	if currentContent != "" {
		userPrompt += fmt.Sprintf("\n\n修正対象ファイル (%s) の現在の内容:\n```\n%s\n```", step.TargetFile, currentContent)
	}

	// コードファイルの場合はプロジェクト構造も注入
	if isCodeFile(step.TargetFile) {
		userPrompt += collectCodebaseContext()
	}

	e.Logger.Info("Running fixer", "model", modelSpec)

	result, err := provider.Generate(fixerPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("fixer generation failed: %w", err)
	}

	e.Logger.Info("Fixer response received", "length", len(result))

	// result からコードブロックを抽出して対象ファイルに適用する
	fixCode := result
	// 対象ファイルの拡張子に応じて優先的に対応する言語を探す
	if strings.HasSuffix(step.TargetFile, ".go") {
		if extracted, err := fs.ExtractCodeBlock(result, "go"); err == nil {
			fixCode = extracted
		} else if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
			fixCode = extracted
		} else {
			return fmt.Errorf("fixer output does not contain a valid code block for %s", step.TargetFile)
		}
	} else {
		if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
			fixCode = extracted
		}
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

	if fs.BranchExists(branchName) {
		e.Logger.Info("Branch already exists, switching", "name", branchName)
		if err := fs.SwitchBranch(branchName); err != nil {
			return fmt.Errorf("failed to switch to existing branch %q: %w", branchName, err)
		}
	} else {
		e.Logger.Info("Creating new branch", "name", branchName)
		if err := fs.CreateBranch(branchName); err != nil {
			return fmt.Errorf("failed to create branch %q: %w", branchName, err)
		}
	}

	// ブランチ名をテンプレート変数に登録（{{branch_name}} で参照可能）
	e.Vars["branch_name"] = branchName

	return nil
}

// runSingleReview は単一の review ステップを実行し、結果を返します。
// 戻り値:
//   - approved: ユーザーが approve したか
//   - fixApplied: 修正が適用されたか（レビューゲートの再実行判定に使用）
//   - err: エラー
func (e *Engine) runSingleReview(step Step) (approved bool, fixApplied bool, err error) {
	modelSpec := ResolveModelSpec(step.Provider, step.Model)
	e.Logger.Info("Running review", "id", step.ID, "reviewer", modelSpec)

	provider, err := llm.GetProvider(modelSpec)
	if err != nil {
		return false, false, fmt.Errorf("failed to get reviewer: %w", err)
	}

	reviewPrompt := ""
	if step.ReviewPromptFile != "" {
		reviewPrompt, err = fs.ReadFile(step.ReviewPromptFile)
		if err != nil {
			return false, false, fmt.Errorf("failed to read review prompt: %w", err)
		}
	}

	targetContent, err := fs.ReadFile(step.TargetFile)
	if err != nil {
		return false, false, fmt.Errorf("failed to read target file for review: %w", err)
	}

	userPrompt := fmt.Sprintf("以下のファイルをレビューしてください:\n\nファイル: %s\n\n内容:\n%s",
		step.TargetFile, targetContent)

	// ワークフローコンテキスト（前ステップの成果要約）を注入してレビューの観点を補強
	reviewContext := e.buildContextSummary()
	if reviewContext != "" {
		userPrompt += reviewContext
	}

	e.Logger.Info("Running reviewer", "model", modelSpec)
	result, err := provider.Generate(reviewPrompt, userPrompt)
	if err != nil {
		return false, false, fmt.Errorf("review generation failed: %w", err)
	}

	// レビュー結果を表示
	fmt.Println("\n================================================================================")
	fmt.Printf("🔍 %s レビュー結果\n", step.ID)
	fmt.Println("================================================================================")
	fmt.Println(result)
	fmt.Println("================================================================================")

	// 履歴を保存（ブランチ名があればサブディレクトリに整理）
	reviewDir := "docs/reviews"
	if bn, ok := e.Vars["branch_name"]; ok && bn != "" {
		reviewDir = fmt.Sprintf("docs/%s/reviews", bn)
	}
	reviewLogPath := fmt.Sprintf("%s/review-%s.md", reviewDir, step.ID)
	if err := fs.WriteFile(reviewLogPath, result); err != nil {
		e.Logger.Warn("Failed to save review log", "error", err)
	}

	// ユーザーに承認を求める
	for {
		fmt.Print("\nこの内容で Approve しますか？ [a]pprove / [f]ix & approve / [r]eject / [q]uit: ")

		var input string
		if _, err := fmt.Scanln(&input); err != nil {
			e.Logger.Warn("Failed to read input", "error", err)
			continue
		}
		input = strings.ToLower(strings.TrimSpace(input))

		switch input {
		case "a", "approve":
			e.Logger.Info("Review approved (no fix needed)", "id", step.ID)
			return true, false, nil

		case "f", "fix":
			// 修正を適用してから approve
			e.Logger.Info("Applying fix based on review feedback", "id", step.ID)
			if err := e.runFixer(step, result); err != nil {
				return false, false, fmt.Errorf("fixer failed: %w", err)
			}
			e.Logger.Info("Fix applied and approved", "id", step.ID)
			return true, true, nil // approved=true, fixApplied=true → ゲートが全レビュー再実行

		case "r", "reject":
			e.Logger.Warn("Review rejected by user", "id", step.ID)
			return false, false, nil

		case "q", "quit":
			os.Exit(0)

		default:
			fmt.Println("無効な入力です。'a', 'f', 'r', または 'q' を入力してください。")
		}
	}
}

// runGitPush は git_push タイプのステップを実行します。
// プッシュ前に未コミットの変更を自動コミットし、成果物の漏れを防ぎます。
func (e *Engine) runGitPush(step Step) error {
	remote := step.Remote
	if remote == "" {
		remote = "origin"
	}

	// プッシュ前に未コミットの変更を自動コミット（成果物の漏れ防止）
	if _, err := fs.AutoCommit("auto-commit-before-push"); err != nil {
		e.Logger.Warn("Auto-commit before push failed (continuing)", "error", err)
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

// resolveVars はテンプレート変数 {{key}} をエンジンの Vars マップで置換します。
func (e *Engine) resolveVars(s string) string {
	for k, v := range e.Vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

// resolveStepPaths はステップ内のファイルパス・コマンドのテンプレート変数を解決します。
func (e *Engine) resolveStepPaths(step *Step) {
	step.InputFile = e.resolveVars(step.InputFile)
	step.OutputFile = e.resolveVars(step.OutputFile)
	step.TargetFile = e.resolveVars(step.TargetFile)
	step.SystemPromptFile = e.resolveVars(step.SystemPromptFile)
	step.FixerPromptFile = e.resolveVars(step.FixerPromptFile)
	step.ReviewPromptFile = e.resolveVars(step.ReviewPromptFile)
	step.BranchNameFile = e.resolveVars(step.BranchNameFile)
	step.Command = e.resolveVars(step.Command)
}

// codeExtensions はコード生成時に自動コンテキスト注入の対象となる拡張子です。
var codeExtensions = []string{".go", ".py", ".ts", ".js", ".rs", ".java"}

// isCodeFile はファイルパスがコードファイルかどうかを判定します。
func isCodeFile(path string) bool {
	for _, ext := range codeExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// collectCodebaseContext はプロジェクトの既存コード構造を収集し、
// LLM に渡すコンテキスト文字列を返します。
// これにより LLM が存在しないパッケージをインポートする幻覚を防止します。
func collectCodebaseContext() string {
	var sb strings.Builder
	sb.WriteString("\n\n---\n# 既存プロジェクト構造（自動収集・変更不可）\n")
	sb.WriteString("以下のパッケージとファイルが実際に存在します。存在しないパッケージをインポートしないでください。\n\n")

	// go.mod の内容（モジュールパスと依存関係）
	if data, err := os.ReadFile("go.mod"); err == nil {
		sb.WriteString("## go.mod\n```\n")
		sb.WriteString(string(data))
		sb.WriteString("```\n\n")
	}

	// Go ファイルの一覧（テストファイルは除外）
	sb.WriteString("## Go ソースファイル一覧\n```\n")
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || (path != "." && strings.HasPrefix(base, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			sb.WriteString(path + "\n")
		}
		return nil
	})
	sb.WriteString("```\n")

	return sb.String()
}
