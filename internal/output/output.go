package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON outputs data as JSON
func PrintJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// PrintTable outputs data as a human-readable table
func PrintTable(data interface{}) error {
	// For v0.1, simple JSON pretty-print as fallback
	// Can be enhanced with table library later
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

