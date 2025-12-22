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

// PrintError outputs an error message to stderr
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

// PrintSuccess outputs a success message
func PrintSuccess(msg string) {
	fmt.Println(msg)
}
