package common

import "strings"

func SplitCommand(command string) []string {
	words := strings.Split(command, " ")

	return words
}
