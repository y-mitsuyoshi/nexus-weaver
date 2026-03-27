package main

// GenerateResult は、何らかの生成処理の結果を表現するための構造体です。
// 生成されたコンテンツ、処理の成功/失敗ステータス、および失敗時のエラーメッセージを保持します。
type GenerateResult struct {
	Content      string // 生成された主要なコンテンツ。通常、成功時に格納されます。
	IsSuccess    bool   // 生成処理が成功した場合は true、失敗した場合は false です。
	ErrorMessage string // 生成処理が失敗した場合のエラーメッセージ。成功時は空文字列であるべきです。
}

// NewSuccessResult は、成功した生成結果を表す GenerateResult インスタンスを作成します。
// content: 成功時に生成された文字列コンテンツ。
func NewSuccessResult(content string) GenerateResult {
	return GenerateResult{
		Content:   content,
		IsSuccess: true,
		// 成功時は ErrorMessage は空文字列のままです。
	}
}

// NewFailureResult は、失敗した生成結果を表す GenerateResult インスタンスを作成します。
// errorMessage: 処理が失敗した理由を説明するメッセージ。
func NewFailureResult(errorMessage string) GenerateResult {
	return GenerateResult{
		// 失敗時は Content は空文字列のままです。
		IsSuccess:    false,
		ErrorMessage: errorMessage,
	}
}