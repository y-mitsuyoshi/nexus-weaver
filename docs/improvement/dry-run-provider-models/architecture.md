## 3. コンポーネント責務 (Component Responsibilities)

*   **CLI (cmd/nexus-weaver)**
    *   ユーザーコマンドライン引数の解析（`--dry-run` など）。
    *   `internal/engine` パッケージの呼び出し。
    *   `internal/engine` から返されたドライランサマリーの表示。
*   **Engine (internal/engine)**
    *   **Runner**: ワークフロー定義のロードと解析、各ステップの実行フロー制御。
        *   各ステップ実行時に使用するLLMプロバイダを決定し、`LLMProvider` インターフェースを介してLLMを呼び出す。
        *   LLM呼び出し時に、使用するLLMプロバイダ名とモデル名を`LLMProvider`インターフェースから取得し、ステップ実行結果と共に保持する。
        *   ワークフロー全体の実行完了後、収集したステップ情報（LLM情報を含む）に基づき、ドライランサマリーを生成する。
    *   **Workflow**: ワークフロー構造の定義、ステップごとの実行ロジックのカプセル化。
*   **LLM Provider Interface (internal/llm/provider.go)**
    *   LLMとの具体的な通信処理を抽象化。
    *   各LLMプロバイダ（Gemini, LocalQwenなど）は本インターフェースを実装する。
    *   **新責務**: 実装しているLLMプロバイダの名前と使用モデル名を返す機能を提供する。
*   **Workflow Definition (workflows/*.yml)**
    *   ワークフローの構造、各ステップの定義、LLM設定（プロバイダ、モデル、プロンプトなど）を記述。

## 4. データフローとシーケンス (Data Flow)

主要なユースケース：ユーザーがドライランを実行し、LLM情報を含むサマリーが生成されるまで。