package nfo

import (
	"bufio"
	"fmt"
	"os"
)

// GetInput prompts the user for input and returns the cleaned string.
// It reads from standard input until a newline character is encountered.
func GetInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf(prompt)
	response, _ := reader.ReadString('\n')

	return cleanInput(response)
}
