# nexus-weaver

`nexus-weaver` は、YAML で定義したワークフローに従って複数の LLM を切り替えながら処理を進める、Go 製の CLI オーケストレーターです。

PRD 作成、設計、実装、テスト実行、コマンド実行といった工程を 1 本のパイプラインとして表現できます。

## できること

- YAML でワークフローを定義する
- ステップごとに `gemini-cli` / `copilot-cli` / `local-qwen` を切り替える
- 入力ファイルを読み、LLM の出力をファイルへ保存する
- テストコマンドをリトライ付きで実行する
- 任意のシェルコマンドをワークフローの最後に実行する

## ディレクトリ構成

```text
nexus-weaver/
├── cmd/nexus-weaver/     # CLI エントリーポイント
├── internal/
│   ├── engine/           # ワークフローパーサー & 実行エンジン
│   ├── fs/               # ファイル I/O と Git ユーティリティ
│   └── llm/              # LLM プロバイダ実装
├── prompts/              # システムプロンプト (.md)
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
- `local-qwen` を使う場合: OpenAI 互換 API が稼働していること。エンドポイントは環境変数 `LOCAL_QWEN_ENDPOINT` で指定可能（デフォルト: `http://localhost:11434/v1/chat/completions`）。

## セットアップ

### ローカルで実行する

```bash
go build -o nexus-weaver ./cmd/nexus-weaver
```

## 基本的な使い方

### 1. ワークフローを確認する

まずはドライランで YAML の読み込みとバリデーションだけを確認します。

```bash
./nexus-weaver --workflow workflows/workflow.yml --dry-run
```

### 2. ワークフローを実行する

```bash
./nexus-weaver --workflow workflows/workflow.yml
```

## ワークフローの書き方

ワークフローは `name` と `steps` で構成されます。

サンプル (`workflows/workflow.yml`):

```yaml
name: "Feature Development Pipeline"
steps:
  - id: "generate_branch_name"
    type: "llm_task"
    agent_role: "ReleaseEngineer"
    model: "gemini-cli"
    system_prompt_file: "./prompts/branch_namer.md"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/branch_name.txt"

  - id: "branch_creation"
    type: "git_branch"
    branch_name_file: "./docs/branch_name.txt"

  - id: "prd_generation"
    type: "llm_task"
    agent_role: "ProductManager"
    model: "gemini-cli"
    system_prompt_file: "./prompts/pm.md"
    input_file: "./inbox/idea.txt"
    output_file: "./docs/prd.md"

  - id: "architecture_design"
    type: "llm_task"
    agent_role: "Architect"
    model: "gemini-cli"
    system_prompt_file: "./prompts/architect.md"
    input_file: "./docs/prd.md"
    output_file: "./docs/architecture.md"

  - id: "implementation"
    type: "llm_task"
    agent_role: "Engineer"
    model: "local-qwen"
    system_prompt_file: "./prompts/engineer.md"
    input_file: "./docs/architecture.md"
    output_file: "./cmd/nexus-weaver/main.go"

  - id: "test_and_fix_loop"
    type: "loop"
    max_retries: 3
    command: "go test ./..."
    fixer_model: "gemini-cli"
    fixer_prompt_file: "./prompts/fixer.md"
    target_file: "./cmd/nexus-weaver/main.go"

  - id: "code_review"
    type: "review"
    reviewer_model: "gemini-cli"
    review_prompt_file: "./prompts/reviewer.md"
    target_file: "./cmd/nexus-weaver/main.go"

  - id: "qa_review"
    type: "review"
    reviewer_model: "gemini-cli"
    review_prompt_file: "./prompts/qa.md"
    target_file: "./cmd/nexus-weaver/main.go"

  - id: "push_to_remote"
    type: "git_push"
    remote: "origin"

  - id: "create_pr"
    type: "command_task"
    command: "gh pr create --title 'Auto PR' --body-file ./docs/prd.md"
```

## ステップ種別

### `llm_task`

LLM にテキスト生成を依頼し、その結果を `output_file` に保存します。出力が `.go` や `.yml` の場合、Markdown のコードブロックを自動抽出します。

### `loop`

テストや検証コマンドを実行し、失敗した場合に Fixer モデルを呼び出します。
各リトライ前に `AutoCommit` が行われ、最終失敗時には `Rollback` が実行されます。

### `git_branch`

新しい Git ブランチを作成し、チェックアウトします。保護ブランチでの直接実行を防ぐため、これらのブランチから開始する場合はこのステップが必須です。
`branch_name` または `branch_name_file` でブランチ名を指定します。

### `review`

LLM によるコードレビューを実行し、ユーザーに対話的な承認を求めます。承認されるまで次のステップには進みません。判定には `MUST`, `IMO`, `nits` タグが使用されます。

### `git_push`

現在のブランチを指定されたリモートにプッシュします。

### `command_task`

任意のシェルコマンドを実行します。

## License

See [LICENSE](LICENSE) file.
