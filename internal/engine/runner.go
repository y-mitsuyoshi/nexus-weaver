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
	sb.WriteString("これらの決定事項・成果物との一貫性を保ってください。\n")
	sb.WriteString("**重要: このセクションは内部メタデータです。あなたの出力にこの情報を転記・引用しないでください。**\n\n")

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

// extractLastNonEmptyLine はテキストの最後の非空行を返します。
// LLM出力に余分なフォーマット行が含まれる場合の安全策として使用します。
func extractLastNonEmptyLine(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
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
		var result string
		switch step.Type {
		case "llm_task":
			result, err = e.runLLMTask(step)
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
		} else {
			// llm_task の結果を蓄積（要約を切り詰めて後続ステップへの過剰な伝搬を防止）
			e.addStepResult(StepResult{
				StepID:     step.ID,
				StepType:   step.Type,
				AgentRole:  step.AgentRole,
				OutputFile: step.OutputFile,
				Summary:    summarizeOutput(result, 500),
				Success:    true,
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
// 3. プロジェクト構造を自動注入（全ステップ共通）
// 4. LLM に Generate を依頼
// 5. 結果を出力ファイルに書き込み（指定がある場合）
func (e *Engine) runLLMTask(step Step) (string, error) {
	modelSpec := ResolveModelSpec(step.Provider, step.Model)
	provider, err := llm.GetProvider(modelSpec)
	if err != nil {
		return "", fmt.Errorf("failed to get provider: %w", err)
	}

	// システムプロンプトの読み込み
	systemPrompt := ""
	if step.SystemPromptFile != "" {
		systemPrompt, err = fs.ReadFile(step.SystemPromptFile)
		if err != nil {
			return "", fmt.Errorf("failed to read system prompt: %w", err)
		}
	}

	// 入力ファイルの読み込み
	inputData := ""
	if step.InputFile != "" {
		inputData, err = fs.ReadFile(step.InputFile)
		if err != nil {
			return "", fmt.Errorf("failed to read input file: %w", err)
		}
	}

	// ワークフローコンテキスト（前ステップの成果要約）をシステムプロンプトに注入
	// ユーザー入力と分離することで、LLM がメタデータを出力に混入するリスクを低減する
	contextSummary := e.buildContextSummary()
	if contextSummary != "" {
		systemPrompt += contextSummary
	}

	// 参照ドキュメント（PRD・設計書）の全文をシステムプロンプトに注入
	// 実装が要件定義・設計と整合していることを担保する
	refContext := e.buildReferenceContext(step)
	if refContext != "" {
		systemPrompt += refContext
	}

	// プロジェクト構造をコンテキストとして自動注入
	// ドキュメント生成（.md 出力）では注入しない — go.mod やファイル一覧は
	// PRD・設計書の品質に寄与せず、LLM の注意を本来のタスクから逸らす原因になる
	if !isDocumentFile(step.OutputFile) {
		ctx := collectCodebaseContext()
		inputData += ctx
		e.Logger.Debug("Auto-injected codebase context", "output_file", step.OutputFile)
	}

	if isCodeFile(step.OutputFile) {
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
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// 出力バリデーション（ガードレール）: 空出力を拒否
	trimmed := strings.TrimSpace(result)
	if len(trimmed) == 0 {
		return "", fmt.Errorf("LLM returned empty output for step %q", step.ID)
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
				return "", fmt.Errorf("failed to extract Go code block from LLM output for step %q (output may not contain valid code fences)", step.ID)
			}
		} else if strings.HasSuffix(step.OutputFile, ".yml") || strings.HasSuffix(step.OutputFile, ".yaml") {
			if extracted, err := fs.ExtractCodeBlock(result, "yaml"); err == nil {
				outputData = extracted
			} else if extracted, err := fs.ExtractCodeBlock(result, "yml"); err == nil {
				outputData = extracted
			} else if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
				outputData = extracted
			}
		} else if strings.HasSuffix(step.OutputFile, ".md") {
			// Markdown ファイルの場合: LLM が ```markdown で囲んだ場合のみアンラップする。
			// 汎用コードブロック抽出（lang=""）は使わない。Markdown 文書は本文中に
			// コードブロック例を含むことが多く、汎用抽出すると最初のコードブロック
			// （例: ユースシナリオの出力例）だけが保存されてしまうため。
			if extracted, err := fs.ExtractCodeBlock(result, "markdown"); err == nil {
				outputData = extracted
			} else if extracted, err := fs.ExtractCodeBlock(result, "md"); err == nil {
				outputData = extracted
			}
			// Markdown ヘッダ（# で始まる行）より前のゴミ（実行ログ、メタデータ等）を除去する。
			// LLM がシステムプロンプトの指示に反してヘッダ前に余計なテキストを出力した場合の安全策。
			outputData = stripBeforeFirstHeading(outputData)
		} else {
			if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
				outputData = extracted
			}
		}

		if err := fs.WriteFile(step.OutputFile, outputData); err != nil {
			return "", fmt.Errorf("failed to write output: %w", err)
		}
		e.Logger.Info("Output written", "file", step.OutputFile)
	}

	return result, nil
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
		if isCodeFile(step.TargetFile) {
			contextDesc = "以下のレビュー指摘を受けました。コードを修正してください。"
		} else {
			contextDesc = fmt.Sprintf("以下のレビュー指摘を受けました。指摘内容を反映して %s を改善してください。\n指摘への回答ではなく、指摘を踏まえた修正済みドキュメント全文を出力してください。", step.TargetFile)
		}
	}

	userPrompt := fmt.Sprintf("%s\n\n指摘内容・エラー出力:\n%s",
		contextDesc, errorOrReviewOutput)

	// ワークフローコンテキスト（前ステップの成果要約）を注入して修正の方向性を伝える
	fixerContext := e.buildContextSummary()
	if fixerContext != "" {
		userPrompt += fixerContext
	}

	// 参照ドキュメント（PRD・設計書）の全文を注入して修正が要件・設計と整合するよう誘導
	refContext := e.buildReferenceContext(step)
	if refContext != "" {
		userPrompt += refContext
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
	} else if strings.HasSuffix(step.TargetFile, ".md") {
		if extracted, err := fs.ExtractCodeBlock(result, "markdown"); err == nil {
			fixCode = extracted
		} else if extracted, err := fs.ExtractCodeBlock(result, "md"); err == nil {
			fixCode = extracted
		} else if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
			fixCode = extracted
		}
	} else if strings.HasSuffix(step.TargetFile, ".yml") || strings.HasSuffix(step.TargetFile, ".yaml") {
		if extracted, err := fs.ExtractCodeBlock(result, "yaml"); err == nil {
			fixCode = extracted
		} else if extracted, err := fs.ExtractCodeBlock(result, "yml"); err == nil {
			fixCode = extracted
		} else if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
			fixCode = extracted
		}
	} else {
		if extracted, err := fs.ExtractCodeBlock(result, ""); err == nil {
			fixCode = extracted
		}
	}

	// ドキュメントファイルの場合、上書き前に現在のバージョンをバックアップ（積み上げ式保全）
	if !isCodeFile(step.TargetFile) && currentContent != "" {
		backupPath := generateVersionedBackupPath(step.TargetFile)
		if err := fs.WriteFile(backupPath, currentContent); err != nil {
			e.Logger.Warn("Failed to save document version backup", "path", backupPath, "error", err)
		} else {
			e.Logger.Info("Document version preserved", "backup", backupPath)
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

	// 1. 直前の llm_task などで生成された結果があれば、それを優先して使用する
	if branchName == "" {
		for i := len(e.StepResults) - 1; i >= 0; i-- {
			res := e.StepResults[i]
			// 直近の llm_task (ReleaseEngineer) の要約または全文をブランチ名候補とする
			// Summary ではなく、最新の Vars や StepResults から取得するように拡張可能
			if res.AgentRole == "ReleaseEngineer" && res.Success {
				branchName = extractLastNonEmptyLine(res.Summary)
				e.Logger.Info("Using branch name from previous step result", "name", branchName)
				break
			}
		}
	}

	// 2. ファイル指定がある場合はファイルから読み込む（既存挙動の互換性）
	if branchName == "" && step.BranchNameFile != "" {
		content, err := fs.ReadFile(step.BranchNameFile)
		if err != nil {
			return fmt.Errorf("failed to read branch name file %s: %w", step.BranchNameFile, err)
		}
		// ファイルから最後の非空行を取得（LLM出力に余分なフォーマットが含まれる場合の防御）
		branchName = extractLastNonEmptyLine(content)
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

	// ブランチ名ファイルをブランチ成果物ディレクトリに移動
	if step.BranchNameFile != "" {
		destDir := filepath.Join("docs", branchName)
		destFile := filepath.Join(destDir, filepath.Base(step.BranchNameFile))
		if err := os.MkdirAll(destDir, 0755); err != nil {
			e.Logger.Warn("Failed to create branch docs directory", "dir", destDir, "error", err)
		} else if err := os.Rename(step.BranchNameFile, destFile); err != nil {
			e.Logger.Warn("Failed to move branch name file", "from", step.BranchNameFile, "to", destFile, "error", err)
		} else {
			e.Logger.Info("Branch name file moved", "to", destFile)
		}
	}

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

	// 参照ドキュメント（PRD・設計書）の全文を注入してレビューが要件・設計と整合しているか確認
	refContext := e.buildReferenceContext(step)
	if refContext != "" {
		userPrompt += refContext
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
	// 同一ファイルに追記することで過去のレビューラウンドの結果も保持する
	reviewDir := "docs/reviews"
	if bn, ok := e.Vars["branch_name"]; ok && bn != "" {
		reviewDir = fmt.Sprintf("docs/%s/reviews", bn)
	}
	reviewLogPath := fmt.Sprintf("%s/review-%s.md", reviewDir, step.ID)
	entry := fmt.Sprintf("\n---\n## Review round\n\n%s\n", result)
	if err := fs.AppendFile(reviewLogPath, entry); err != nil {
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
	for i := range step.ReferenceFiles {
		step.ReferenceFiles[i] = e.resolveVars(step.ReferenceFiles[i])
	}
}

// documentExtensions はドキュメント生成と判定する拡張子です。
var documentExtensions = []string{".md", ".txt", ".rst"}

// isDocumentFile は出力先がドキュメントファイルかどうかを判定します。
func isDocumentFile(path string) bool {
	for _, ext := range documentExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// stripBeforeFirstHeading は Markdown テキストから最初の見出し（# で始まる行）より前の
// 余計なテキストを除去します。LLM が見出し前に実行ログやメタデータを出力した場合の安全策です。
// 見出しが見つからない場合は元のテキストをそのまま返します。
func stripBeforeFirstHeading(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			return strings.Join(lines[i:], "\n")
		}
	}
	return text
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

// generateVersionedBackupPath は指定パスに対して未使用のバージョン付きバックアップパスを生成します。
// 例: architecture.md → architecture_v1.md, architecture_v2.md, ...
func generateVersionedBackupPath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for v := 1; ; v++ {
		candidate := fmt.Sprintf("%s_v%d%s", base, v, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// collectAutoReferenceFiles は完了済みステップの成果物から自動的に参照すべきドキュメントを収集します。
// PRD・設計書などのドキュメント出力ファイルの最新版（レビュー・修正反映済み）を返します。
// excludeFiles に指定されたパスは、既に別経路で入力されているため重複を避けて除外されます。
func (e *Engine) collectAutoReferenceFiles(excludeFiles ...string) []string {
	seen := make(map[string]bool)
	for _, f := range excludeFiles {
		if f != "" {
			seen[f] = true
		}
	}

	var refs []string
	for _, r := range e.StepResults {
		if r.OutputFile != "" && r.Success && isDocumentFile(r.OutputFile) {
			if !seen[r.OutputFile] {
				seen[r.OutputFile] = true
				refs = append(refs, r.OutputFile)
			}
		}
	}
	return refs
}

// buildReferenceContext は参照すべきドキュメント（PRD・設計書等）の全文を読み込み、
// コンテキスト文字列として返します。
//
// 参照ドキュメントは以下の2つのソースから自動的に決定されます:
//  1. 自動収集: 完了済みステップのドキュメント出力（最新版 = レビュー・修正反映済み）
//  2. 明示指定: ステップの reference_files フィールド（オーバーライド用）
//
// InputFile / OutputFile / TargetFile は既に別経路で LLM に渡されるため、自動収集から除外されます。
func (e *Engine) buildReferenceContext(step Step) string {
	// 自動収集: 完了済みステップのドキュメント出力（入力・出力・対象と重複するものは除外）
	autoRefs := e.collectAutoReferenceFiles(step.InputFile, step.OutputFile, step.TargetFile)

	// 明示指定 + 自動収集をマージ（重複排除）
	seen := make(map[string]bool)
	var allRefs []string
	for _, ref := range step.ReferenceFiles {
		if !seen[ref] {
			seen[ref] = true
			allRefs = append(allRefs, ref)
		}
	}
	for _, ref := range autoRefs {
		if !seen[ref] {
			seen[ref] = true
			allRefs = append(allRefs, ref)
		}
	}

	if len(allRefs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n---\n# 参照ドキュメント（PRD・設計書）\n")
	sb.WriteString("以下は本ワークフローで作成・レビュー済みの要件定義・設計ドキュメントです。\n")
	sb.WriteString("実装・レビュー・修正はこれらのドキュメントに定義された要件・設計方針と整合している必要があります。\n\n")

	for _, refPath := range allRefs {
		content, err := fs.ReadFile(refPath)
		if err != nil {
			e.Logger.Warn("Failed to read reference file (skipping)", "path", refPath, "error", err)
			continue
		}
		sb.WriteString(fmt.Sprintf("## 参照: %s\n\n%s\n\n", refPath, content))
	}

	return sb.String()
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
