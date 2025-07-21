// nfo package provides logging and output capabilities, including local log files with rotation and simply output to termianl.
package nfo

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"strings"
	"syscall"
)

// cancel is a channel used to signal cancellation of a process.
var cancel = make(chan struct{})

// getEscape returns a function that, when called, restores the terminal
// state to what it was before the function was called.
func getEscape() func() {
	s, _ := terminal.GetState(int(syscall.Stdin))
	return func() { terminal.Restore(int(syscall.Stdin), s) }
}

// NeedAnswer repeatedly requests an answer from a function until a
// non-empty string is returned.
func NeedAnswer(prompt string, request func(prompt string) string) (output string) {
	for output = request(prompt); output == ""; output = request(prompt) {
	}
	return output
}

// PressEnter prints a prompt and waits for the user to press Enter.
// It masks the input to prevent it from being displayed on the screen.
func PressEnter(prompt string) {
	unesc := Defer(getEscape())
	defer unesc()

	fmt.Printf("\r%s", prompt)

	var blank_line []rune
	for range prompt {
		blank_line = append(blank_line, ' ')
	}
	terminal.ReadPassword(int(syscall.Stdin))
	fmt.Printf("\r%s\r", string(blank_line))
}

// GetSecret prompts the user for a secret string.
// It disables terminal echoing while reading input.
func GetSecret(prompt string) string {
	unesc := Defer(getEscape())
	defer unesc()

	fmt.Printf(prompt)
	resp, _ := terminal.ReadPassword(int(syscall.Stdin))
	output := cleanInput(string(resp))
	fmt.Printf("\n")
	return output
}

// GetConfirm prompts the user with a message and returns true if they enter "y" or "yes", and false if they enter "n" or "no".
func GetConfirm(prompt string) bool {
	for {
		resp := GetInput(fmt.Sprintf("%s (y/n): ", prompt))
		resp = strings.ToLower(resp)
		if resp == "y" || resp == "yes" {
			return true
		} else if resp == "n" || resp == "no" {
			return false
		}
		continue
	}
}

// Confirms a default answer to a boolean question from the user.
// Prompts the user for confirmation with a default answer (Y/n or y/N).
func ConfirmDefault(prompt string, default_answer bool) bool {
	for {
		var question string
		if default_answer {
			question = fmt.Sprintf("%s (Y/n): ", prompt)
		} else {
			question = fmt.Sprintf("%s (y/N): ", prompt)
		}
		resp := GetInput(question)
		resp = strings.ToLower(resp)
		resp = strings.TrimSpace(resp)
		switch resp {
		case "y":
			return true
		case "n":
			return false
		case "":
			return default_answer
		}
		continue
	}
}

// cleanInput removes newline and carriage return characters from a string
// and trims any leading/trailing whitespace.
func cleanInput(input string) (output string) {
	var output_bytes []rune
	for _, v := range input {
		if v == '\n' || v == '\r' {
			continue
		}
		output_bytes = append(output_bytes, v)
	}
	return strings.TrimSpace(string(output_bytes))
}
