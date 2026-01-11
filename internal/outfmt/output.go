package outfmt

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format represents output format
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// Printer handles formatted output
type Printer struct {
	Format Format
	Writer io.Writer
}

// NewPrinter creates a new printer with the given format
func NewPrinter(format string) *Printer {
	return &Printer{
		Format: Format(format),
		Writer: os.Stdout,
	}
}

// Print outputs data in the configured format
func (p *Printer) Print(data interface{}) error {
	switch p.Format {
	case FormatJSON:
		return p.printJSON(data)
	case FormatYAML:
		return p.printYAML(data)
	default:
		return fmt.Errorf("table format requires PrintTable method")
	}
}

func (p *Printer) printJSON(data interface{}) error {
	encoder := json.NewEncoder(p.Writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (p *Printer) printYAML(data interface{}) error {
	encoder := yaml.NewEncoder(p.Writer)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

// PrintTable prints data in table format
func (p *Printer) PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(p.Writer, 0, 0, 2, ' ', 0)

	// Print headers
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, h)
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, cell)
		}
		fmt.Fprintln(w)
	}

	w.Flush()
}
