package fs

import (
	"strings"
	"testing"
)

func TestExtractCodeBlock_Go(t *testing.T) {
	markdown := "Here is some Go code:\n\n```go\npackage main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\nDone."

	result, err := ExtractCodeBlock(markdown, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "package main") {
		t.Errorf("expected 'package main' in result, got: %s", result)
	}
	if !strings.Contains(result, "fmt.Println") {
		t.Errorf("expected 'fmt.Println' in result, got: %s", result)
	}
}

func TestExtractCodeBlock_Python(t *testing.T) {
	markdown := "```python\ndef hello():\n    print('hello')\n```"

	result, err := ExtractCodeBlock(markdown, "python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "def hello()") {
		t.Errorf("expected 'def hello()' in result, got: %s", result)
	}
}

func TestExtractCodeBlock_YAML(t *testing.T) {
	markdown := "Config:\n\n```yaml\nname: test\nsteps:\n  - id: step1\n```\n"

	result, err := ExtractCodeBlock(markdown, "yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "name: test") {
		t.Errorf("expected 'name: test' in result, got: %s", result)
	}
}

func TestExtractCodeBlock_NotFound(t *testing.T) {
	markdown := "No code blocks here."

	_, err := ExtractCodeBlock(markdown, "go")
	if err == nil {
		t.Fatal("expected error for missing code block, got nil")
	}
}

func TestExtractAllCodeBlocks(t *testing.T) {
	markdown := "Block 1:\n\n```go\npackage a\n```\n\nBlock 2:\n\n```go\npackage b\n```\n"

	results, err := ExtractAllCodeBlocks(markdown, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(results))
	}

	if results[0] != "package a" {
		t.Errorf("expected 'package a', got %q", results[0])
	}
	if results[1] != "package b" {
		t.Errorf("expected 'package b', got %q", results[1])
	}
}

func TestExtractAllCodeBlocks_NotFound(t *testing.T) {
	_, err := ExtractAllCodeBlocks("no blocks", "go")
	if err == nil {
		t.Fatal("expected error for missing code blocks, got nil")
	}
}
