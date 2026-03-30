[DRY-RUN] ワークフロー: my-workflow (3 ステップ)

  Step 1: generate_branch_name
    Provider : copilot_cli
    Model    : claude-sonnet-4.6
    Input    : inbox/idea.txt
    Output   : (stdout)

  Step 2: code_generation
    Provider : gemini_cli
    Model    : gemini-2.0-flash
    Input    : (stdout from step 1)
    Output   : internal/engine/runner.go

  Step 3: review
    Provider : copilot_cli
    Model    : (unset → default が使用されます)
    Input    : internal/engine/runner.go
    Output   : (stdout)

[使用LLM一覧]
  - copilot_cli / claude-sonnet-4.6
  - gemini_cli  / gemini-2.0-flash
  - copilot_cli / (default)