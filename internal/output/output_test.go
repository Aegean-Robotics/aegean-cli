package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	for _, good := range []string{"text", "json", "yaml"} {
		if err := Validate(good); err != nil {
			t.Errorf("Validate(%q): %v", good, err)
		}
	}
	if err := Validate("xml"); err == nil {
		t.Error("Validate(xml): want error")
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, map[string]any{"a": 1, "b": "two"}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"a": 1`) || !strings.Contains(out, `"b": "two"`) {
		t.Errorf("got %q", out)
	}
}

func TestYAML(t *testing.T) {
	var buf bytes.Buffer
	if err := YAML(&buf, map[string]any{"name": "ada", "age": 36}); err != nil {
		t.Fatalf("YAML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "age: 36") || !strings.Contains(out, "name: ada") {
		t.Errorf("yaml output: %q", out)
	}
}

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	err := Table(&buf, []string{"ID", "NAME"}, [][]string{
		{"1", "alice"},
		{"22", "bob"},
	})
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 lines, got %d: %q", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "ID") || !strings.HasPrefix(lines[2], "1 ") || !strings.HasPrefix(lines[3], "22") {
		t.Errorf("table alignment off: %q", out)
	}
}
