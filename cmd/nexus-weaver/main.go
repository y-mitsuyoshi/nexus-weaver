package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/y-mitsuyoshi/nexus-weaver/internal/engine"
)

// ビルド時に -ldflags で埋め込まれるバージョン情報
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// CLI引数の定義
	workflowPath := flag.String("workflow", "", "ワークフロー定義YAMLファイルのパス（省略時は自動探索）")
	verbose := flag.Bool("verbose", false, "詳細ログの出力を有効にする")
	dryRun := flag.Bool("dry-run", false, "ワークフローの読み込みと検証のみ実行（ステップは実行しない）")
	showVersion := flag.Bool("version", false, "バージョン情報を表示して終了")
	initProject := flag.Bool("init", false, "カレントディレクトリに .nexus/ 設定ディレクトリの雛形を生成")
	flag.Parse()

	// バージョン表示
	if *showVersion {
		fmt.Printf("nexus-weaver %s (commit: %s)\n", version, commit)
		return
	}

	// ログレベルの設定
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// プロジェクト初期化モード
	if *initProject {
		if err := runInit(logger); err != nil {
			logger.Error("Failed to initialize project", "error", err)
			os.Exit(1)
		}
		return
	}

	// ワークフローパスの解決
	resolvedPath, err := resolveWorkflowPath(*workflowPath)
	if err != nil {
		logger.Error("Failed to find workflow file", "error", err)
		fmt.Fprintln(os.Stderr, "\nヒント: 'nexus-weaver --init' で .nexus/ ディレクトリを作成するか、--workflow でパスを指定してください")
		os.Exit(1)
	}

	logger.Info("nexus-weaver starting",
		"version", version,
		"workflow", resolvedPath,
		"dry_run", *dryRun,
	)

	// ワークフローの読み込み
	wf, err := engine.LoadWorkflow(resolvedPath)
	if err != nil {
		logger.Error("Failed to load workflow", "error", err)
		os.Exit(1)
	}

	logger.Info("Workflow loaded successfully",
		"name", wf.Name,
		"steps", len(wf.Steps),
	)

	// ステップ一覧の表示
	for i, step := range wf.Steps {
		logger.Info(fmt.Sprintf("  Step %d", i+1),
			"id", step.ID,
			"type", step.Type,
			"model", step.Model,
		)
	}

	// ドライランモードの場合は実行せずに終了
	if *dryRun {
		logger.Info("Dry run mode: skipping execution")
		return
	}

	// ワークフロー実行
	eng := engine.NewEngine(logger)
	if err := eng.Run(wf); err != nil {
		logger.Error("Workflow execution failed", "error", err)
		os.Exit(1)
	}

	logger.Info("All steps completed successfully")
}

// defaultWorkflowPaths はワークフロー定義ファイルの探索順序を定義します。
// nexus-weaver はカレントディレクトリ（=対象リポジトリ）を起点として、
// 以下の順序でワークフローファイルを探索します。
var defaultWorkflowPaths = []string{
	".nexus/workflow.yml",
	".nexus/workflow.yaml",
	"workflows/workflow.yml",
	"workflows/workflow.yaml",
}

// resolveWorkflowPath はワークフローファイルのパスを解決します。
// 明示的にパスが指定されている場合はそのまま返し、
// 未指定の場合はカレントディレクトリからデフォルトの候補を順に探索します。
func resolveWorkflowPath(explicit string) (string, error) {
	// 明示的に指定された場合はそのまま返す
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("指定されたワークフローファイルが見つかりません: %s: %w", explicit, err)
		}
		return explicit, nil
	}

	// デフォルトの候補を順に探索
	for _, candidate := range defaultWorkflowPaths {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("ワークフローファイルが見つかりません。探索パス: %v", defaultWorkflowPaths)
}

// runInit はカレントディレクトリに .nexus/ 設定ディレクトリの雛形を生成します。
func runInit(logger *slog.Logger) error {
	// .nexus ディレクトリの作成
	dirs := []string{
		".nexus",
		".nexus/prompts",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("ディレクトリ %s の作成に失敗: %w", dir, err)
		}
		logger.Info("Created directory", "path", dir)
	}

	// ワークフロー雛形
	workflowTemplate := `name: "My Project Pipeline"
steps:
  # 1. フィーチャーブランチを作成
  - id: "branch_creation"
    type: "git_branch"
    branch_name: "feature/my-feature"

  # 2. PRD を生成
  - id: "prd_generation"
    type: "llm_task"
    agent_role: "ProductManager"
    model: "gemini-cli"
    system_prompt_file: ".nexus/prompts/pm.md"
    input_file: "inbox/idea.txt"
    output_file: "docs/prd.md"

  # 3. 実装
  - id: "implementation"
    type: "llm_task"
    agent_role: "Engineer"
    model: "gemini-cli"
    system_prompt_file: ".nexus/prompts/engineer.md"
    input_file: "docs/prd.md"
    output_file: "src/main.go"

  # 4. テスト & 自動修正ループ
  - id: "test_and_fix"
    type: "loop"
    max_retries: 3
    command: "go test ./..."
    fixer_model: "gemini-cli"
    fixer_prompt_file: ".nexus/prompts/fixer.md"
    target_file: "src/main.go"

  # 5. コードレビュー
  - id: "code_review"
    type: "review"
    reviewer_model: "gemini-cli"
    review_prompt_file: ".nexus/prompts/reviewer.md"
    target_file: "src/main.go"
    max_retries: 3

  # 6. プッシュ
  - id: "push_to_remote"
    type: "git_push"
    remote: "origin"
`

	workflowPath := ".nexus/workflow.yml"
	if _, err := os.Stat(workflowPath); err == nil {
		logger.Warn("Workflow file already exists, skipping", "path", workflowPath)
	} else {
		if err := os.WriteFile(workflowPath, []byte(workflowTemplate), 0644); err != nil {
			return fmt.Errorf("ワークフロー雛形の書き込みに失敗: %w", err)
		}
		logger.Info("Created workflow template", "path", workflowPath)
	}

	// .gitignore への追記提案
	logger.Info("✅ .nexus/ ディレクトリを作成しました")
	logger.Info("次のステップ:")
	logger.Info("  1. .nexus/workflow.yml を編集してワークフローを定義")
	logger.Info("  2. .nexus/prompts/ にプロンプトファイルを配置")
	logger.Info("  3. nexus-weaver --dry-run で検証")
	logger.Info("  4. nexus-weaver で実行")

	return nil
}
