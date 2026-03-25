# nexus-weaver

`nexus-weaver` は、YAML で定義したワークフローに従って複数の LLM を切り替えながら処理を進める、Go 製の CLI オーケストレーターです。

PRD 作成、設計、実装、テスト実行、コマンド実行といった工程を 1 本のパイプラインとして表現できます。

## できること

- YAML でワークフローを定義する
- ステップごとに `gemini-cli` / `copilot-cli` / `local-qwen` を切り替える
- 入力ファイルを読み、LLM の出力をファイルへ保存する
- テストコマンドをリトライ付きで実行する
- 任意のシェルコマンドをワークフローの最後に実行する

## 現在の実装状況

`loop` ステップはテスト失敗時に Fixer モデルを呼び出し、Fixer の応答からフェンスドコードブロック（```...```）を抽出して、ワークフローで指定した `target_file` に書き込む処理が実装されています。`loop` を使用する場合は必ず `target_file` を指定してください。



## ディレクトリ構成

```text
nexus-weaver/
├── cmd/nexus-weaver/     # CLI エントリーポイント
├── internal/
│   ├── engine/           # ワークフローパーサー & 実行エンジン
│   ├── fs/               # ファイル I/O と Git ユーティリティ
│   └── llm/              # LLM プロバイダ実装
├── prompts/              # システムプロンプト
├── workflows/            # ワークフロー定義 YAML
└── docs/                 # 生成物や設計書
```

## 前提条件

### 共通

- Go 1.24 以上
- Git

### 利用する LLM に応じて必要なもの

- `gemini-cli` を使う場合: `gemini` コマンドが実行できること
- `copilot-cli` を使う場合: `copilot` コマンドが実行できること
- `local-qwen` を使う場合: OpenAI 互換 API が `http://localhost:11434/v1/chat/completions` で応答すること

`local-qwen` は `internal/llm/provider.go` で上記エンドポイントに固定されています。

## セットアップ

### ローカルで実行する

```bash
go build -o nexus-weaver ./cmd/nexus-weaver
```

### Docker Compose で実行する

```bash
docker compose build
```

`docker-compose.yml` はカレントディレクトリを `/app` にマウントし、`network_mode: "host"` を使用しています（Linux 環境でのみ有効です）。これはコンテナからホストの `localhost:11434`（local-qwen）へ到達するためです。Mac/Windows の Docker Desktop では host ネットワークが制限されるため、別のネットワーク方法が必要です。

## 基本的な使い方

### 1. ワークフローを確認する

まずはドライランで YAML の読み込みとバリデーションだけを確認します。

```bash
./nexus-weaver --workflow workflows/workflow.yml --dry-run
```

Docker Compose の場合:

```bash
docker compose run --rm nexus-weaver --dry-run
```

### 2. ワークフローを実行する

```bash
./nexus-weaver --workflow workflows/workflow.yml
```

Docker Compose の場合:

```bash
docker compose run --rm nexus-weaver
```

### 3. 詳細ログを有効にする

```bash
./nexus-weaver --workflow workflows/workflow.yml --verbose
```

## CLI オプション

| Flag | Default | Description |
|------|---------|-------------|
| `--workflow` | `workflows/workflow.yml` | ワークフロー定義 YAML のパス |
| `--verbose` | `false` | 詳細ログを出力する |
| `--dry-run` | `false` | 読み込みと検証のみ行い、実行はしない |

## ワークフローの書き方

ワークフローは `name` と `steps` で構成されます。

サンプル:

```yaml
name: "Feature Development Pipeline"
steps:
  - id: "prd_generation"
    type: "llm_task"
    agent_role: "ProductManager"
    model: "gemini-cli"
    system_prompt_file: "./prompts/pm.txt"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/prd.md"

  - id: "architecture_design"
    type: "llm_task"
    agent_role: "Architect"
    model: "gemini-cli"
    system_prompt_file: "./prompts/architect.txt"
    input_file: "./docs/prd.md"
    output_file: "./docs/architecture.md"

  - id: "implementation"
    type: "llm_task"
    agent_role: "Engineer"
    model: "local-qwen"
    system_prompt_file: "./prompts/engineer.txt"
    input_file: "./docs/architecture.md"
    output_file: "./src/main.go"

  - id: "test_and_fix_loop"
    type: "loop"
    max_retries: 3
    command: "go test ./..."
    fixer_model: "gemini-cli"
    fixer_prompt_file: "./prompts/fixer.txt"
    target_file: "./src/main.go"

  - id: "create_pr"
    type: "command_task"
    command: "gh pr create --title 'Auto PR' --body-file ./docs/prd.md"
```

## ステップ種別

### `llm_task`

LLM にテキスト生成を依頼し、その結果を `output_file` に保存します。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `llm_task`
- `agent_role`: ログ出力用の役割名
- `model`: `gemini-cli` / `copilot-cli` / `local-qwen`
- `system_prompt_file`: システムプロンプトのファイル
- `input_file`: 入力ファイル
- `output_file`: 出力先ファイル

### `loop`

テストや検証コマンドを実行し、失敗した場合に Fixer モデルを呼び出します。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `loop`
- `command`: 実行するコマンド
- `max_retries`: 最大試行回数
- `fixer_model`: 失敗時に呼ぶモデル
- `fixer_prompt_file`: Fixer 用システムプロンプト

注意点:

- 各リトライ前に Git 自動コミット（AutoCommit）が行われます。これは変更を保護するための安全装置です
- 最終リトライまで失敗した場合、直前のコミットに戻すために Git の Rollback（`git reset --hard HEAD~1`）が実行されます。これらは破壊的な操作になり得るため注意してください
- Fixer の応答は `target_file` に自動適用されます（フェンスドコードブロックが抽出されます）

### `command_task`

任意のシェルコマンドを実行します。PR 作成や補助スクリプト実行などに使えます。

主なフィールド:

- `id`: ステップの一意な識別子
- `type`: `command_task`
- `command`: `sh -c` で実行するコマンド

## 実行の流れ

1. CLI がワークフロー YAML を読み込む
2. 各ステップの定義を検証する
3. `llm_task` は入力ファイルを読み、LLM の出力をファイルへ保存する
4. `loop` はコマンドを実行し、失敗時は Fixer モデルを呼ぶ
5. `command_task` は指定コマンドをそのまま実行する

## よくある実行例

### PRD 生成だけを試したい

ワークフローを最小構成にして `llm_task` を 1 つだけ置くと、テキスト生成ツールとして使えます。

### ローカル LLM でコード生成したい

`implementation` ステップの `model` を `local-qwen` にし、ローカルの OpenAI 互換 API を起動した状態で実行します。

### GitHub CLI と組み合わせて PR を作りたい

最後に `command_task` で `gh pr create ...` を実行すると、生成した PRD や要約を使って PR 作成を自動化できます。

## 注意事項

- `command_task` は `sh -c` で実行されるため、コマンド内容は慎重に管理してください
- `AutoCommit`（自動コミット）と `Rollback`（直前コミットへのリセット）は `loop` ステップ内で使用されます。最終リトライで失敗した場合に Rollback が実行されます。これらはワークツリーを上書きする可能性があるため、未コミットの重要な変更がないことを確認してください。
- Docker イメージには `gemini` や `copilot` CLI 自体は含まれていません。必要に応じてホストまたはイメージ側で用意してください

## テスト

```bash
go test ./... -v
```

## License

See [LICENSE](LICENSE) file.
