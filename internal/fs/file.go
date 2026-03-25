package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ReadFile は指定されたパスのファイルを読み込み、内容を文字列で返します。
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return string(data), nil
}

// WriteFile は指定されたパスにコンテンツを書き込みます。
// 必要に応じて親ディレクトリを自動的に作成します。
func WriteFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}

// ExtractCodeBlock は Markdown テキストからフェンスドコードブロックを抽出します。
// lang に対応する言語のコードブロック（例: ```go ... ```）からコード部分だけを返します。
// 複数のブロックが存在する場合は最初のものを返します。
func ExtractCodeBlock(markdown, lang string) (string, error) {
	// ```lang と ``` で囲まれた部分を抽出する正規表現
	pattern := fmt.Sprintf("(?s)```%s\\s*\\n(.*?)\\n\\s*```", regexp.QuoteMeta(lang))
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(markdown)
	if len(matches) < 2 {
		return "", fmt.Errorf("no %s code block found in markdown", lang)
	}

	return strings.TrimSpace(matches[1]), nil
}

// ExtractAllCodeBlocks は Markdown テキストから指定言語の全てのコードブロックを抽出します。
func ExtractAllCodeBlocks(markdown, lang string) ([]string, error) {
	pattern := fmt.Sprintf("(?s)```%s\\s*\\n(.*?)\\n\\s*```", regexp.QuoteMeta(lang))
	re := regexp.MustCompile(pattern)

	allMatches := re.FindAllStringSubmatch(markdown, -1)
	if len(allMatches) == 0 {
		return nil, fmt.Errorf("no %s code blocks found in markdown", lang)
	}

	results := make([]string, len(allMatches))
	for i, match := range allMatches {
		results[i] = strings.TrimSpace(match[1])
	}
	return results, nil
}
