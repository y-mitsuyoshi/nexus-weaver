package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/y-mitsuyoshi/nexus-weaver/internal/engine"
)

// ビルド時に -ldflags で埋め込まれるバージョン情報
var (
	version = "dev"
	commit  = "unknown"
)

// inputFiles は --input フラグで複数回指定可能なファイルパスリストです。
type inputFiles []string

func (i *inputFiles) String() string { return strings.Join(*i, ", ") }
func (i *inputFiles) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	// CLI引数の定義
	workflowPath := flag.String("workflow", "", "ワークフロー定義YAMLファイルのパス（省略時は自動探索）")
	verbose := flag.Bool("verbose", false, "詳細ログの出力を有効にする")
	dryRun := flag.Bool("dry-run", false, "ワークフローの読み込みと検証のみ実行（ステップは実行しない）")
	showVersion := flag.Bool("version", false, "バージョン情報を表示して終了")
	initProject := flag.Bool("init", false, "カレントディレクトリに .nexus/ 設定ディレクトリの雛形を生成")

	// プロンプトとモデルの定義（エイリアス付き）
	promptPtr := flag.String("prompt", "", "ワークフローへの初期プロンプト（命令）。inbox/idea.txt に保存される")
	pAlias := flag.String("p", "", "alias for --prompt (one-shot mode)")
	modelPtr := flag.String("model", "gemini-cli", "使用するLLMモデル名（デフォルト: gemini-cli）")
	mAlias := flag.String("m", "", "alias for --model")

	var inputs inputFiles
	flag.Var(&inputs, "input", "参照ファイルのパス（複数指定可）。プロンプトと結合されて inbox/idea.txt に保存される")
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
	// ロガー出力は os.Stderr に向ける（stdout を結果表示用に空着けておく）
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// プロンプトとモデルの解決
	prompt := *promptPtr
	if prompt == "" {
		prompt = *pAlias
	}
	// 位置引数もプロンプトとして扱う（フラグが未指定の場合）
	if prompt == "" && flag.NArg() > 0 {
		prompt = strings.Join(flag.Args(), " ")
	}

	model := *modelPtr
	if *mAlias != "" {
		model = *mAlias
	}

	// プロジェクト初期化モード
	if *initProject {
		if err := runInit(logger); err != nil {
			logger.Error("Failed to initialize project", "error", err)
			os.Exit(1)
		}
		return
	}

	// プロンプトが指定された場合、入力ファイルを生成
	if prompt != "" {
		if err := saveInitialPrompt(logger, prompt, inputs); err != nil {
			logger.Error("Failed to save initial prompt", "error", err)
			os.Exit(1)
		}
	}

	// ワークフローパスの解決
	resolvedPath, err := resolveWorkflowPath(*workflowPath)
	if err != nil {
		// ワークフローが見つからないがプロンプトがある場合、ワンショット実行モードとして振る舞う
		if prompt != "" {
			logger.Info("No workflow file found. Entering one-shot execution mode", "model", model)
			wf := &engine.Workflow{
				Name: "One-shot Task",
				Steps: []engine.Step{
					{
						ID:        "oneshot",
						Type:      "llm_task",
						Model:     model,
						InputFile: "inbox/idea.txt",
					},
				},
			}
			eng := engine.NewEngine(logger)
			if err := eng.Run(wf); err != nil {
				logger.Error("One-shot execution failed", "error", err)
				os.Exit(1)
			}
			logger.Info("One-shot mode completed")
			return
		}

		logger.Error("Failed to find workflow file", "error", err)
		fmt.Fprintln(os.Stderr, "\nヒント: 'nexus-weaver --init' で .nexus/ ディレクトリを作成するか、--workflow でパスを指定してください")
		fmt.Fprintln(os.Stderr, "または 'nexus-weaver -p \"質問内容\"' でワンショット実行も可能です")
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

// saveInitialPrompt は CLI 引数で渡されたプロンプトと参照ファイルの内容を
// ワークフローの入力ファイル（inbox/idea.txt）に保存します。
//
// 生成されるファイルのフォーマット:
//
//	# タスク
//	<プロンプトの内容>
//
//	# 参照ファイル（--input で指定された場合）
//	## path/to/file.go
//	```
//	<ファイルの内容>
//	```
func saveInitialPrompt(logger *slog.Logger, prompt string, refs []string) error {
	const defaultInputPath = "inbox/idea.txt"

	// ディレクトリの作成
	dir := filepath.Dir(defaultInputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("ディレクトリ %s の作成に失敗: %w", dir, err)
	}

	// プロンプトの構築
	var sb strings.Builder
	sb.WriteString("# タスク\n\n")
	sb.WriteString(prompt)
	sb.WriteString("\n")

	// 参照ファイルの付加
	if len(refs) > 0 {
		sb.WriteString("\n# 参照ファイル\n")
		for _, ref := range refs {
			content, err := os.ReadFile(ref)
			if err != nil {
				return fmt.Errorf("参照ファイル %s の読み込みに失敗: %w", ref, err)
			}
			sb.WriteString(fmt.Sprintf("\n## %s\n\n```\n%s\n```\n", ref, strings.TrimRight(string(content), "\n")))
			logger.Debug("Attached reference file", "path", ref, "size", len(content))
		}
	}

	// 書き込み
	if err := os.WriteFile(defaultInputPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("プロンプトの保存に失敗: %w", err)
	}

	logger.Info("Initial prompt saved",
		"path", defaultInputPath,
		"prompt_length", len(prompt),
		"reference_files", len(refs),
	)
	return nil
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
    provider: "gemini-cli"
    system_prompt_file: ".nexus/prompts/pm.md"
    input_file: "inbox/idea.txt"
    output_file: "docs/prd.md"

  # 3. 実装
  - id: "implementation"
    type: "llm_task"
    agent_role: "Engineer"
    provider: "gemini-cli"
    system_prompt_file: ".nexus/prompts/engineer.md"
    input_file: "docs/prd.md"
    output_file: "src/main.go"

  # 4. テスト & 自動修正ループ
  - id: "test_and_fix"
    type: "loop"
    max_retries: 3
    command: "go test ./..."
    provider: "gemini-cli"
    fixer_prompt_file: ".nexus/prompts/fixer.md"
    target_file: "src/main.go"

  # 5. コードレビュー
  - id: "code_review"
    type: "review"
    provider: "gemini-cli"
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
