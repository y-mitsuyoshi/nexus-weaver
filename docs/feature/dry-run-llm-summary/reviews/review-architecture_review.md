
---
## Review round

## レビュー結果

---

### MUST① — ワークフロー実行コンテキストの混入

**該当箇所**: 設計書末尾の `# ワークフロー実行コンテキスト（前ステップの成果）` セクション全体

設計書本文に実行ログ・メタデータが転記されています。セクション内に「**重要: このセクションは内部メタデータです。あなたの出力にこの情報を転記・引用しないでください。**」と明記されているにもかかわらず、設計書ファイルとして保存されています。

**対処**: このセクションを設計書から完全に削除してください。設計書には設計判断・方針のみ記述するべきです。

---

### MUST② — セクション 7（ディレクトリ構成案）が実質空白

**該当箇所**: `## 7. ディレクトリ構成案`

```
internal/

cmd/
```

2行しかなく、どのパッケージに何を配置するかの情報がゼロです。設計書のセクションとして機能していません。

**対処**: 最低限以下の粒度で記述してください。

```
internal/
  llm/
    resolver.go        # LLMConfigResolver インターフェース + デフォルト実装
  engine/
    workflow.go        # Step 構造体（Provider/Model フィールド追加）
    dryrun.go          # DryRunHandler, DryRunStep, DryRunFormatter
cmd/
  root.go              # --dry-run フラグ追加
```

---

### IMO — `LLMConfigResolver` インターフェースのシグネチャ設計

**該当箇所**: セクション 6

```go
ResolveProvider(step Step, globalCfg GlobalConfig) string
```

`Step` と `GlobalConfig` という2つの上位型を引数に取る設計は、`LLMConfigResolver` が Engine 層と Config 層の両方に依存することを意味します。テスト時にモックが2つ必要になり、ADR-002 で謳うテスト容易性の利点が薄れます。

**代替案**: プリミティブ値を渡す設計にすることで依存を断ち切れます。

```go
Resolve(stepProvider, stepModel, defaultProvider, defaultModel string) (provider, model string)
```

呼び出し側（`DryRunHandler`）がフィールドアクセスの責任を持てばよく、`LLMConfigResolver` はフォールバックロジックのみに専念できます。

---

### nits — セクション 3 の表にコード上の識別子が混入

**該当箇所**: コンポーネント表の「LLMProvider Interface」行

> `ProviderRegistry / GetProvider`

設計書のコンポーネント表には概念レベルの説明を書くべきで、`GetProvider()` のような具体的な関数名はコードに委ねてください。「プロバイダー実装のルックアップ機構」程度の記述が適切です。
