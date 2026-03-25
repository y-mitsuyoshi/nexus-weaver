package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/y-mitsuyoshi/nexus-weaver/internal/engine"
)

func main() {
	// CLI引数の定義
	workflowPath := flag.String("workflow", "workflows/workflow.yml", "ワークフロー定義YAMLファイルのパス")
	verbose := flag.Bool("verbose", false, "詳細ログの出力を有効にする")
	dryRun := flag.Bool("dry-run", false, "ワークフローの読み込みと検証のみ実行（ステップは実行しない）")
	flag.Parse()

	// ログレベルの設定
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	logger.Info("nexus-weaver starting",
		"workflow", *workflowPath,
		"dry_run", *dryRun,
	)

	// ワークフローの読み込み
	wf, err := engine.LoadWorkflow(*workflowPath)
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
