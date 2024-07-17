package filesystem

import (
	"path"
	"strings"
)

// ? Clean the input path for the filesystem.
// * Paths should be not be other than ASCII characters.
func Clean(input string) string {
	var output string = input

	switch output {
	case "":
		output = "/"
	default:
		if !strings.HasPrefix(output, "/") {
			output = LeadSlash(output)
		}

		if !strings.HasSuffix(output, "/") {
			output = TrailSlash(output)
		}
	}

	return path.Clean(output)
}

func UntrailSlash(input string) string {
	return strings.TrimSuffix(input, "/")
}

func UnleadSlash(input string) string {
	return strings.TrimPrefix(input, "/")
}

func TrailSlash(input string) string {
	return input + "/"
}

func LeadSlash(input string) string {
	return "/" + input
}
