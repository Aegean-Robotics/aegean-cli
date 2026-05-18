// Package output renders values in text|json|yaml.
//
// "text" is intentionally hand-rolled per type (table-ish, no library
// dependency). "json" is encoding/json with two-space indent. "yaml" is a
// minimal flat-map encoding sufficient for the v0.1.0 types — we'll swap to
// gopkg.in/yaml.v3 once we have a use case that needs nested maps in YAML.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	FormatText = "text"
	FormatJSON = "json"
	FormatYAML = "yaml"
)

// Validate returns an error if format is not one of the supported values.
func Validate(format string) error {
	switch format {
	case FormatText, FormatJSON, FormatYAML:
		return nil
	}
	return fmt.Errorf("unsupported output format %q (use text|json|yaml)", format)
}

// JSON encodes v as indented JSON.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// YAML encodes v as flat YAML. Only string/number/bool/slice values are
// supported; structs go through json marshal first so field tags apply.
func YAML(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return err
	}
	return writeYAML(w, generic, 0)
}

func writeYAML(w io.Writer, v any, indent int) error {
	prefix := strings.Repeat("  ", indent)
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := val[k]
			switch child.(type) {
			case map[string]any, []any:
				if _, err := fmt.Fprintf(w, "%s%s:\n", prefix, k); err != nil {
					return err
				}
				if err := writeYAML(w, child, indent+1); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%s%s: %s\n", prefix, k, yamlScalar(child)); err != nil {
					return err
				}
			}
		}
	case []any:
		for _, item := range val {
			switch item.(type) {
			case map[string]any, []any:
				if _, err := fmt.Fprintf(w, "%s-\n", prefix); err != nil {
					return err
				}
				if err := writeYAML(w, item, indent+1); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%s- %s\n", prefix, yamlScalar(item)); err != nil {
					return err
				}
			}
		}
	default:
		if _, err := fmt.Fprintln(w, yamlScalar(v)); err != nil {
			return err
		}
	}
	return nil
}

func yamlScalar(v any) string {
	if v == nil {
		return "null"
	}
	s := fmt.Sprintf("%v", v)
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#\n") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// Table renders rows as a fixed-width text table. headers and each row must
// have the same length.
func Table(w io.Writer, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	writeRow := func(cells []string) error {
		parts := make([]string, len(cells))
		for i, cell := range cells {
			if i == len(cells)-1 {
				parts[i] = cell
			} else {
				parts[i] = padRight(cell, widths[i])
			}
		}
		_, err := fmt.Fprintln(w, strings.Join(parts, "  "))
		return err
	}

	if err := writeRow(headers); err != nil {
		return err
	}
	dividers := make([]string, len(headers))
	for i := range headers {
		dividers[i] = strings.Repeat("-", widths[i])
	}
	if err := writeRow(dividers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeRow(row); err != nil {
			return err
		}
	}
	return nil
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
