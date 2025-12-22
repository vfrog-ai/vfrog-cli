package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// PrintJSON outputs data as JSON
func PrintJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// PrintError outputs an error message to stderr
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

// PrintSuccess outputs a success message
func PrintSuccess(msg string) {
	fmt.Println(msg)
}

// Table represents a table for output
type Table struct {
	headers []string
	rows    [][]string
	marker  int // index of selected row (-1 for none)
}

// NewTable creates a new table with the given headers
func NewTable(headers ...string) *Table {
	return &Table{
		headers: headers,
		rows:    make([][]string, 0),
		marker:  -1,
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(values ...string) {
	t.rows = append(t.rows, values)
}

// AddRowWithMarker adds a row and marks it as selected
func (t *Table) AddRowWithMarker(selected bool, values ...string) {
	t.rows = append(t.rows, values)
	if selected {
		t.marker = len(t.rows) - 1
	}
}

// Print outputs the table in kubectl-style format
func (t *Table) Print() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print headers with marker column
	fmt.Fprintf(w, "  \t%s\n", strings.Join(t.headers, "\t"))

	// Print rows
	for i, row := range t.rows {
		marker := " "
		if i == t.marker {
			marker = "✓"
		}
		fmt.Fprintf(w, "%s\t%s\n", marker, strings.Join(row, "\t"))
	}

	w.Flush()
}
