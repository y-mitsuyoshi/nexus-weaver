mermaid
C4Context
    title nexus-weaver dry-run LLM情報サマリー機能拡張

    System_Boundary(nexus_weaver_sys, "nexus-weaver CLI") {
        Container(cli, "CLI Application", "Goアプリケーション", "dry-runコマンドを受け付け、ワークフローエンジンを起動")
        Container(engine_runner, "Workflow Engine Runner", "Goコンポーネント", "ワークフロー定義を解釈し、各ステップを実行またはドライランを調整")
        Container(engine_workflow, "Workflow Step Executor", "Goコンポーネント", "個々のワークフローステップのロジックを管理し、LLMプロバイダと連携")
        Container(llm_provider_iface, "LLM Provider Interface", "Goインターフェース", "LLMプロバイダの抽象化。名前とモデル情報取得メソッドを含む")
        Container(gemini_cli_impl, "Gemini CLI Provider", "Goコンポーネント", "Gemini CLIの具体的なLLMプロバイダ実装")
        Container(copilot_cli_impl, "Copilot CLI Provider", "Goコンポーネント", "Copilot CLIの具体的なLLMプロバイダ実装")
        Container(local_qwen_impl, "Local Qwen Provider", "Goコンポーネント", "Local Qwenの具体的なLLMプロバイダ実装")
        Container(workflow_definition, "Workflow Definition", "YAMLファイル", "ワークフローの構造とステップを定義")
    }

    Rel(cli, engine_runner, "dry-runコマンドを発行")
    Rel(engine_runner, workflow_definition, "ワークフロー定義を読み込み")
    Rel(engine_runner, engine_workflow, "各ワークフローステップのドライランを委譲")
    Rel(engine_workflow, llm_provider_iface, "LLMプロバイダ情報をインターフェース経由で要求")
    Rel_Impl(gemini_cli_impl, llm_provider_iface, "実装")
    Rel_Impl(copilot_cli_impl, llm_provider_iface, "実装")
    Rel_Impl(local_qwen_impl, llm_provider_iface, "実装")
    Rel(llm_provider_iface, engine_workflow, "LLMプロバイダ名とモデル情報を返却")
    Rel(engine_workflow, engine_runner, "ステップ実行結果とLLM情報を含むサマリーを返却")
    Rel(engine_runner, cli, "最終的なドライランサマリーを出力")