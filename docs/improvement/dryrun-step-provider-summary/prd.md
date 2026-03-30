go
	// ステップ一覧の表示
	for i, step := range wf.Steps {
		logger.Info(fmt.Sprintf("  Step %d", i+1),
			"id", step.ID,
			"type", step.Type,
			"provider", step.Provider,
			"model", step.Model,
		)
	}

	// ドライランモードの場合は実行せずに終了
	if *dryRun {
		logger.Info("Dry run mode: skipping execution")
		return
	}