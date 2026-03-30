markdown
# nexus-weaver dry-run LLM情報サマリー — アーキテクチャ設計書（改訂版）

mermaid
C4Context
    title nexus-weaver dry-run LLM情報サマリー機能拡張（改訂版）

    System_Boundary(nexus_weaver_sys, "nexus-weaver CLI") {
        Container(cli, "CLI アプリケーション", "Goバイナリ", "ユーザー入力を受け付け、dry-runを開始")
        Container(engine-runner, "Workflow Engine Runner", "Goコンポーネント", "ワークフロー定義を読み取り、各ステップのドライランを制御")
        Container(engine-workflow, "Workflow Step Executor", "Goコンポーネント", "各ステップのロジックを評価し、LLMプロバイダからドライラン用メタを取得")
        Container(llm-provider-iface, "LLM Provider Interface", "Goインターフェース", "ドライラン用メタ（ネットワーク不要）を返すことを保証")
        Container(gemini-cli-impl, "Gemini CLI Provider（実装）", "Goコンポーネント", "ローカル/オフラインで返せるメタを実装")
        Container(copilot-cli-impl, "Copilot CLI Provider（実装）", "Goコンポーネント", "オフラインメタまたはキャッシュを返却")
        Container(local-qwen-impl, "Local Qwen Provider（実装）", "Goコンポーネント", "即時応答・キャッシュ戦略を持つローカル実装")
        Artifact(workflow-definition, "Workflow Definition (YAML)", "ファイル／アーティファクト", "ワークフロー定義")
    }

    Rel(cli, engine-runner, "dry-runコマンドを発行")
    Rel(engine-runner, workflow-definition, "定義を読み込む")
    Rel(engine-runner, engine-workflow, "各ステップのドライランを委譲")
    Rel(engine-workflow, llm-provider-iface, "DryRunMetadata(ctx) を要求（ネットワーク禁止）")
    Rel_Impl(gemini-cli-impl, llm-provider-iface, "実装")
    Rel_Impl(copilot-cli-impl, llm-provider-iface, "実装")
    Rel_Impl(local-qwen-impl, llm-provider-iface, "実装")
    Rel(llm-provider-iface, engine-workflow, "ModelInfo（ネットワーク不要メタ）を返却（出所情報含む）")
    Rel(engine-workflow, engine-runner, "ステップサマリー（JSON/MD）を返却")
    Rel(engine-runner, cli, "最終サマリーを出力 (--format=json|md)")

---

## 目的（要約）
dry-run 実行時に、ワークフローで使用される LLM プロバイダとモデル情報の「安全で検証可能な」サマリーを出力する。重要要件は以下：

- dry-run 中に外部LLMへネットワークコールを**絶対に行わない**ことを設計上保証する  
- 資格情報やAPIキー等の機密情報をサマリーに含めない／出力しないことを検証する  
- 取得情報の「出所（取得時刻／キャッシュ有無／失敗理由）」を含め、データ整合性を担保する  
- プロバイダ未応答や権限不足はワークフロー失敗にせず、"unknown"/"warning"フェイルセーフで扱う  
- インターフェースの互換性管理（バージョニング／後方互換ルール）を明確化する

以下、設計詳細。

---

## MUST（必須要件）と設計反映

1. ドライランでのネットワーク禁止（設計・実装）
   - provider インターフェースに必須メソッドを追加：
     - DryRunMetadata(ctx context.Context) (ModelInfo, error)
   - DryRunMetadata は「ネットワーク不要」で返せる情報のみを返すことを実装者に義務付ける（ローカル設定、キャッシュ、ハードコーディングされたメタなど）。
   - remote 実装は、起動/ビルド時に事前取得したオフラインメタやキャッシュを用いる。取得不能な場合は network を使わずに "resolutionStatus" を "unknown" または "warning" として返す。

2. 機密情報の排除と検証
   - ModelInfo やサマリーは**絶対に**資格情報（APIキー、シークレット、トークン、裸の環境変数値、コマンド引数）を含めてはならない。
   - サマリー生成時に RedactCandidates を実行し、赤裸のenv値やコマンド引数が含まれていないことを検証する自動チェックを追加する（ユニット/統合テストで必須）。
   - CI で走る統合テストは、dry-run 実行中に外向きネットワーク接続が発生していないことを検証する（ネットワークダミー/フックで検出）。

3. 取得情報の出所（プロバイダメタ）を必須化
   - ModelInfo に以下の出所フィールドを追加：`retrievedAt`（ISO8601、取得時刻またはキャッシュ作成時刻）、`cacheHit`（bool）、`resolutionStatus`（enum: ok/unknown/warning/error）、`failReason`（任意の説明テキスト、機密情報非含有）。
   - これらはサマリー出力と JSON schema に含め、データ整合性とトレーサビリティを確保する。

4. フェイルセーフ挙動
   - プロバイダ未応答や権限不足はワークフローを失敗扱いにしない。engine は該当ステップの ModelInfo.resolutionStatus を unknown/warning に設定し、処理は続行する（ユーザへ注意喚起は行う）。
   - クリティカルな失敗のみ error とし、明確に区別する。

5. Context とタイムアウト
   - すべての provider メソッドは context.Context を受け取り、タイムアウトキャンセルに従うこと。タイムアウト時は安全な代替値（例: resolutionStatus=unknown, retrievedAt=nil）を返す。

6. 互換性とバージョニング
   - provider インターフェースにバージョン情報を付与する：`ProviderAPIVersion() string`（例 `"v1"`）。
   - 後方互換ルール：
     - 新メソッド追加は minor バージョンで許容するが、既存実装と互換性を保つためにデフォルトの shim 実装（ラッパー）を提供するか、コンパイル時に明示的な実装差分を検出してマイグレーションガイダンスを出す。
     - 重大な破壊的変更は major バージョンを上げる。engine は ProviderAPIVersion をチェックして互換性警告を出す。

---

## インターフェース定義（例：Go擬似コード）

以下は設計指針を反映したインターフェース例。実際の実装は `internal/llm/provider.go` を更新すること。