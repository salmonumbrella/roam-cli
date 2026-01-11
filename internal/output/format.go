package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"
)

// Format represents the output format type.
type Format string

const (
	// FormatText is human-readable key-value format (default).
	FormatText Format = "text"
	// FormatJSON is pretty-printed JSON format.
	FormatJSON Format = "json"
	// FormatNDJSON is newline-delimited JSON format.
	FormatNDJSON Format = "ndjson"
	// FormatTable is tabular format for lists.
	FormatTable Format = "table"
	// FormatYAML is YAML format.
	FormatYAML Format = "yaml"
)

// ParseFormat converts a string to a Format type.
// Empty string defaults to FormatText.
// Returns error if the format is invalid.
func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case FormatText, "":
		return FormatText, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatNDJSON:
		return FormatNDJSON, nil
	case FormatTable:
		return FormatTable, nil
	case FormatYAML:
		return FormatYAML, nil
	default:
		return "", errors.New("invalid --output format (expected text|json|ndjson|table|yaml)")
	}
}

// IsStructured reports whether the format is machine-readable structured output.
func IsStructured(format Format) bool {
	switch format {
	case FormatJSON, FormatNDJSON, FormatYAML:
		return true
	default:
		return false
	}
}

// Printer handles output formatting across different formats.
type Printer struct {
	w      io.Writer
	format Format
}

// NewPrinter creates a new Printer that writes to w in the given format.
func NewPrinter(w io.Writer, format Format) *Printer {
	return &Printer{
		w:      w,
		format: format,
	}
}

// Print outputs data in the configured format.
// For single objects: JSON or text key-value display.
// For slices: JSON array or table with headers.
func (p *Printer) Print(ctx context.Context, data interface{}) error {
	if data == nil {
		return nil
	}

	data = ApplyAgentOptions(ctx, data)

	switch p.format {
	case FormatJSON:
		return p.printJSON(ctx, data)
	case FormatNDJSON:
		return p.printNDJSON(ctx, data)
	case FormatYAML:
		return p.printYAML(data)
	case FormatTable:
		return p.printTable(data)
	case FormatText:
		return p.printText(data)
	default:
		return fmt.Errorf("unsupported format: %s", p.format)
	}
}

// printJSON outputs data as pretty-printed JSON.
// If a jq query is present in the context, it filters the output.
func (p *Printer) printJSON(ctx context.Context, data interface{}) error {
	query := QueryFromContext(ctx)
	if query == "" {
		// Normal JSON output
		enc := json.NewEncoder(p.w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// Parse and run jq query
	parsed, err := gojq.Parse(query)
	if err != nil {
		return fmt.Errorf("invalid --query: %w", err)
	}

	code, err := gojq.Compile(parsed)
	if err != nil {
		return fmt.Errorf("invalid --query: %w", err)
	}

	iter := code.Run(data)
	enc := json.NewEncoder(p.w)
	enc.SetEscapeHTML(false)

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			return fmt.Errorf("query error: %w", err)
		}
		if err := enc.Encode(v); err != nil {
			return err
		}
	}

	return nil
}

// printNDJSON outputs data as newline-delimited JSON.
// If a jq query is present in the context, it filters the output.
func (p *Printer) printNDJSON(ctx context.Context, data interface{}) error {
	query := QueryFromContext(ctx)
	enc := json.NewEncoder(p.w)
	enc.SetEscapeHTML(false)

	if query != "" {
		parsed, err := gojq.Parse(query)
		if err != nil {
			return fmt.Errorf("invalid --query: %w", err)
		}

		code, err := gojq.Compile(parsed)
		if err != nil {
			return fmt.Errorf("invalid --query: %w", err)
		}

		iter := code.Run(data)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, isErr := v.(error); isErr {
				return fmt.Errorf("query error: %w", err)
			}
			if err := enc.Encode(v); err != nil {
				return err
			}
		}
		return nil
	}

	v := reflect.ValueOf(data)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		for i := 0; i < v.Len(); i++ {
			if err := enc.Encode(v.Index(i).Interface()); err != nil {
				return err
			}
		}
		return nil
	}

	return enc.Encode(data)
}

// printYAML outputs data as YAML.
func (p *Printer) printYAML(data interface{}) error {
	enc := yaml.NewEncoder(p.w)
	enc.SetIndent(2)
	defer func() { _ = enc.Close() }()
	return enc.Encode(data)
}

// printText outputs data as human-readable text.
// For maps and structs: key-value pairs.
// For slices: one item per line.
// For primitives: direct output.
func (p *Printer) printText(data interface{}) error {
	v := reflect.ValueOf(data)
	if !v.IsValid() {
		return nil
	}

	// Dereference pointers
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		return p.printTextMap(v)
	case reflect.Struct:
		return p.printTextStruct(v)
	case reflect.Slice, reflect.Array:
		return p.printTextSlice(v)
	default:
		_, err := fmt.Fprintln(p.w, v.Interface())
		return err
	}
}

func (p *Printer) printTextMap(v reflect.Value) error {
	if v.Len() == 0 {
		return nil
	}

	// Sort keys for deterministic output
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
	})

	for _, key := range keys {
		val := v.MapIndex(key)
		if !val.IsValid() {
			continue
		}
		_, err := fmt.Fprintf(p.w, "%s: %v\n", key.Interface(), val.Interface())
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Printer) printTextStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		value := v.Field(i)
		if !value.IsValid() {
			continue
		}

		label := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" && parts[0] != "-" {
				label = parts[0]
			}
		}

		// Skip zero values for omitempty
		if strings.Contains(field.Tag.Get("json"), "omitempty") && value.IsZero() {
			continue
		}

		_, err := fmt.Fprintf(p.w, "%s: %v\n", label, value.Interface())
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Printer) printTextSlice(v reflect.Value) error {
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i).Interface()
		_, err := fmt.Fprintln(p.w, item)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Printer) printTable(data interface{}) error {
	// Allow explicit Table type
	if table, ok := data.(Table); ok {
		return p.printTableData(table.Headers, table.Rows)
	}

	v := reflect.ValueOf(data)
	if !v.IsValid() {
		return nil
	}

	// Dereference pointers
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Errorf("table format requires a list of items")
	}

	if v.Len() == 0 {
		return nil
	}

	headers, rows := buildTable(v)
	return p.printTableData(headers, rows)
}

func (p *Printer) printTableData(headers []string, rows [][]string) error {
	w := tabwriter.NewWriter(p.w, 0, 0, 2, ' ', 0)

	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, h)
	}
	fmt.Fprintln(w)

	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, cell)
		}
		fmt.Fprintln(w)
	}

	return w.Flush()
}

func buildTable(v reflect.Value) ([]string, [][]string) {
	first := v.Index(0)
	for first.Kind() == reflect.Ptr {
		if first.IsNil() {
			break
		}
		first = first.Elem()
	}

	// Default for slices of maps or primitives
	if first.Kind() != reflect.Struct {
		headers := []string{"value"}
		rows := make([][]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			rows = append(rows, []string{fmt.Sprint(v.Index(i).Interface())})
		}
		return headers, rows
	}

	type field struct {
		name string
		idx  int
	}
	fields := make([]field, 0, first.NumField())
	for i := 0; i < first.NumField(); i++ {
		f := first.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Name
		if tag := f.Tag.Get("json"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" && parts[0] != "-" {
				name = parts[0]
			}
		}
		fields = append(fields, field{name: name, idx: i})
	}

	headers := make([]string, 0, len(fields))
	for _, f := range fields {
		headers = append(headers, f.name)
	}

	rows := make([][]string, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		row := make([]string, 0, len(fields))
		item := v.Index(i)
		for item.Kind() == reflect.Ptr {
			if item.IsNil() {
				break
			}
			item = item.Elem()
		}
		if item.Kind() != reflect.Struct {
			row = append(row, fmt.Sprint(item.Interface()))
			rows = append(rows, row)
			continue
		}
		for _, f := range fields {
			fieldVal := item.Field(f.idx)
			row = append(row, fmt.Sprint(fieldVal.Interface()))
		}
		rows = append(rows, row)
	}

	return headers, rows
}
