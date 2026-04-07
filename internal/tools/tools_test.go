package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- ReadFile Tests ---

func TestReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	content := "line one\nline two\nline three\nline four\nline five\n"
	path := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(path, []byte(content), 0644)

	tool := &ReadFileTool{}
	args, _ := json.Marshal(readFileArgs{Path: path})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "1\tline one") {
		t.Errorf("expected numbered output, got: %s", result)
	}
	if !strings.Contains(result, "5\tline five") {
		t.Errorf("expected line 5, got: %s", result)
	}
}

func TestReadFileWithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = strings.Repeat("x", 10)
	}
	path := filepath.Join(tmpDir, "big.txt")
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)

	tool := &ReadFileTool{}
	args, _ := json.Marshal(readFileArgs{Path: path, Offset: 10, Limit: 5})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultLines := strings.Split(strings.TrimSpace(result), "\n")
	// Should have 5 content lines + truncation message
	if len(resultLines) < 5 {
		t.Errorf("expected at least 5 lines, got %d", len(resultLines))
	}
	if !strings.HasPrefix(resultLines[0], "11\t") {
		t.Errorf("expected line 11 first, got: %s", resultLines[0])
	}
}

func TestReadFileNotFound(t *testing.T) {
	tool := &ReadFileTool{}
	args, _ := json.Marshal(readFileArgs{Path: "/nonexistent/file.txt"})

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// --- WriteFile Tests ---

func TestWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "output.txt")

	tool := &WriteFileTool{}
	args, _ := json.Marshal(writeFileArgs{Path: path, Content: "hello world"})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("expected success message, got: %s", result)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got: %s", string(data))
	}
}

func TestWriteFileCreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "a", "b", "c", "deep.txt")

	tool := &WriteFileTool{}
	args, _ := json.Marshal(writeFileArgs{Path: path, Content: "deep content"})

	_, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "deep content" {
		t.Errorf("expected 'deep content', got: %s", string(data))
	}
}

// --- EditFile Tests ---

func TestEditFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(path, []byte("foo bar baz"), 0644)

	tool := &EditFileTool{}
	args, _ := json.Marshal(editFileArgs{
		Path:      path,
		OldString: "bar",
		NewString: "qux",
	})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Successfully edited") {
		t.Errorf("expected success message, got: %s", result)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "foo qux baz" {
		t.Errorf("expected 'foo qux baz', got: %s", string(data))
	}
}

func TestEditFileNotFound(t *testing.T) {
	tool := &EditFileTool{}
	args, _ := json.Marshal(editFileArgs{
		Path:      "/nonexistent/file.txt",
		OldString: "x",
		NewString: "y",
	})

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestEditFileNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	tool := &EditFileTool{}
	args, _ := json.Marshal(editFileArgs{
		Path:      path,
		OldString: "nonexistent string",
		NewString: "replacement",
	})

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("expected error for no match")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestEditFileMultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(path, []byte("aaa bbb aaa"), 0644)

	tool := &EditFileTool{}
	args, _ := json.Marshal(editFileArgs{
		Path:      path,
		OldString: "aaa",
		NewString: "ccc",
	})

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("expected error for multiple matches")
	}
	if !strings.Contains(err.Error(), "2 times") {
		t.Errorf("expected '2 times' error, got: %v", err)
	}
}

// --- ListFiles Tests ---

func TestListFiles(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte(""), 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)

	tool := &ListFilesTool{}
	args, _ := json.Marshal(listFilesArgs{Path: tmpDir})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "a.txt") {
		t.Errorf("expected a.txt in listing, got: %s", result)
	}
	if !strings.Contains(result, "b.go") {
		t.Errorf("expected b.go in listing, got: %s", result)
	}
	if !strings.Contains(result, "subdir") {
		t.Errorf("expected subdir in listing, got: %s", result)
	}
}

func TestListFilesWithPattern(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte(""), 0644)

	tool := &ListFilesTool{}
	args, _ := json.Marshal(listFilesArgs{Path: tmpDir, Pattern: "*.go"})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "b.go") {
		t.Errorf("expected b.go in filtered listing, got: %s", result)
	}
	if strings.Contains(result, "a.txt") {
		t.Errorf("did not expect a.txt in *.go filtered listing, got: %s", result)
	}
}

// --- Search Tests ---

func TestSearch(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "code.go"), []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("no match here\n"), 0644)

	tool := &SearchTool{}
	args, _ := json.Marshal(searchArgs{Pattern: "Println", Path: tmpDir})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Println") {
		t.Errorf("expected Println in results, got: %s", result)
	}
	if !strings.Contains(result, "code.go") {
		t.Errorf("expected code.go in results, got: %s", result)
	}
}

func TestSearchNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("nothing relevant\n"), 0644)

	tool := &SearchTool{}
	args, _ := json.Marshal(searchArgs{Pattern: "nonexistent_pattern_xyz", Path: tmpDir})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "No matches") {
		t.Errorf("expected 'No matches' message, got: %s", result)
	}
}

// --- Bash Tests ---

func TestBash(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(bashArgs{Command: "echo hello"})

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(result) != "hello" {
		t.Errorf("expected 'hello', got: %q", result)
	}
}

func TestBashExitCode(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(bashArgs{Command: "exit 42"})

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "exit code 42") {
		t.Errorf("expected exit code 42, got: %v", err)
	}
}

func TestBashTimeout(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(bashArgs{Command: "sleep 10", Timeout: 500}) // 500ms timeout

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

// --- Registry Tests ---

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// Check all built-in tools are registered
	expectedTools := []string{"read_file", "write_file", "edit_file", "list_files", "search", "bash", "web_search"}
	for _, name := range expectedTools {
		_, err := reg.Get(name)
		if err != nil {
			t.Errorf("expected tool %q to be registered: %v", name, err)
		}
	}

	// Unknown tool should error
	_, err := reg.Get("nonexistent_tool")
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestRegistryDefinitions(t *testing.T) {
	reg := NewRegistry()
	defs := reg.Definitions()

	if len(defs) != 7 {
		t.Errorf("expected 7 tool definitions, got %d", len(defs))
	}

	for _, d := range defs {
		if d.Name == "" {
			t.Error("tool definition has empty name")
		}
		if d.Description == "" {
			t.Errorf("tool %q has empty description", d.Name)
		}
		if len(d.Parameters) == 0 {
			t.Errorf("tool %q has empty parameters", d.Name)
		}
	}
}
