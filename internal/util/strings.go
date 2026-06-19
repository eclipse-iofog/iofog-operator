package util

import (
	"strings"
)

func Before(input, substr string) string {
	pos := strings.Index(input, substr)
	if pos == -1 {
		return input
	}

	return input[0:pos]
}

func After(input, substr string) string {
	pos := strings.Index(input, substr)
	if pos == -1 || pos+1 > len(input)-1 {
		return ""
	}

	return input[pos+1:]
}

func TransformImageToARM(image string) string {
	if !strings.Contains(image, ":") {
		return image
	}

	return Before(image, ":") + "-arm:" + After(image, ":")
}
