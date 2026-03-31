package llm

// Unset は provider/model が解決できなかった場合に表示するプレースホルダーです。
const Unset = "(未設定)"

// GlobalConfig はグローバルデフォルトの LLM 設定を保持します。
type GlobalConfig struct {
	Provider string
	Model    string
}

// DefaultProvider はグローバルデフォルトのプロバイダー名を返します。
func (c GlobalConfig) DefaultProvider() string { return c.Provider }

// DefaultModel はグローバルデフォルトのモデル名を返します。
func (c GlobalConfig) DefaultModel() string { return c.Model }

// ConfigResolver はステップとグローバル設定から LLM プロバイダー・モデルを解決します。
//
// 解決の優先順位:
//  1. ステップ個別値（非空文字）
//  2. グローバルデフォルト値（非空文字）
//  3. Unset 定数 "(未設定)"
type ConfigResolver struct{}

// ResolveProvider はステップの provider 文字列を解決します。
func (r ConfigResolver) ResolveProvider(stepProvider string, cfg GlobalConfig) string {
	if stepProvider != "" {
		return stepProvider
	}
	if cfg.DefaultProvider() != "" {
		return cfg.DefaultProvider()
	}
	return Unset
}

// ResolveModel はステップの model 文字列を解決します。
func (r ConfigResolver) ResolveModel(stepModel string, cfg GlobalConfig) string {
	if stepModel != "" {
		return stepModel
	}
	if cfg.DefaultModel() != "" {
		return cfg.DefaultModel()
	}
	return Unset
}
