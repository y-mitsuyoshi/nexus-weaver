---
description: "nexus-weaver のワークフロー YAML を作成・修正するためのスキル"
---

# ワークフロー YAML 作成スキル

## ワークフロー YAML の構造

```yaml
name: "ワークフロー名"
steps:
  - id: "一意のステップID"
    type: "llm_task | loop | command_task"
    # 以下はタイプに応じて設定
```

## ステップタイプ別フィールド

### `llm_task` — LLM に生成を依頼するステップ

```yaml
- id: "prd_generation"
  type: "llm_task"
  agent_role: "ProductManager"       # エージェントのロール名
  model: "gemini-cli"                # gemini-cli | copilot-cli | local-qwen
  system_prompt_file: "./prompts/pm.txt"  # システムプロンプトのファイルパス
  input_file: "./inbox/idea.txt"     # 入力ファイルのパス
  output_file: "./docs/prd.md"       # 出力先ファイルパス
```

**必須フィールド**: `id`, `type`, `model`

### `loop` — テスト実行 & 自動修正ループ

```yaml
- id: "test_and_fix_loop"
  type: "loop"
  max_retries: 3                     # 最大リトライ回数 (>0)
  command: "go test ./..."           # テストコマンド
  fixer_model: "gemini-cli"          # エラー修正に使用するモデル
  fixer_prompt_file: "./prompts/fixer.txt"  # Fixer のプロンプト
```

**必須フィールド**: `id`, `type`, `command`, `max_retries`

### `command_task` — シェルコマンド実行

```yaml
- id: "create_pr"
  type: "command_task"
  command: "gh pr create --title 'Auto PR' --body-file ./docs/prd.md"
```

**必須フィールド**: `id`, `type`, `command`

## バリデーションルール

ワークフロー定義は以下のルールに従う必要があります:

1. `name` は必須
2. `steps` は1つ以上必要
3. 各ステップの `id` は一意であること（重複不可）
4. `type` は `llm_task`, `loop`, `command_task` のいずれか
5. `llm_task` には `model` が必須
6. `loop` には `command` と `max_retries` (>0) が必須
7. `command_task` には `command` が必須

## 利用可能なモデル

| モデル名 | 用途 | 実装 |
|----------|------|------|
| `gemini-cli` | 高推論タスク（PRD, 設計, エラー分析） | Gemini CLI (`os/exec`) |
| `copilot-cli` | 汎用コーディング・エージェント | GitHub Copilot CLI (`os/exec`) |
| `local-qwen` | コスト・速度重視の実装タスク | OpenAI 互換 HTTP API |

## サンプルワークフロー

完全な例は `workflows/workflow.yml` を参照してください。
