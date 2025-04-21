package util

import "regexp"

func SplitString(input string) []string {
	return regexp.MustCompile(`[,\s;]+`).Split(input, -1)
}
