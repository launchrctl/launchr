package test

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"
)

// Constants for repeated string values
const (
	opReplace      = "replace"
	opReplaceRegex = "replace-regex"
	opRemoveLines  = "remove-lines"
	opRemoveRegex  = "remove-regex"
	opExtractLines = "extract-lines"
	opExtractRegex = "extract-regex"
)

// CmdTxtProc provides flexible text processing capabilities
func CmdTxtProc(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("txtproc does not support negation")
	}

	if len(args) < 3 {
		ts.Fatalf("txtproc: usage: txtproc <operation> [args...] <input> <output>")
	}

	operation := args[0]
	var inputFile, outputFile string
	var pattern, replacement string

	switch operation {
	case opReplace:
		if len(args) != 5 {
			ts.Fatalf("txtproc replace: usage: txtproc replace <pattern> <replacement> <input> <output>")
		}
		pattern = args[1]
		replacement = args[2]
		inputFile = args[3]
		outputFile = args[4]

	case opReplaceRegex:
		if len(args) != 5 {
			ts.Fatalf("txtproc replace-regex: usage: txtproc replace-regex <regex> <replacement> <input> <output>")
		}
		pattern = args[1]
		replacement = args[2]
		inputFile = args[3]
		outputFile = args[4]

	case opRemoveLines, opRemoveRegex, opExtractLines, opExtractRegex:
		if len(args) != 4 {
			ts.Fatalf("txtproc %s: usage: txtproc %s <pattern> <input> <output>", operation, operation)
		}
		pattern = args[1]
		inputFile = args[2]
		outputFile = args[3]

	default:
		ts.Fatalf("txtproc: unknown operation %q. Available: replace, replace-regex, remove-lines, remove-regex, extract-lines, extract-regex", operation)
	}

	// Read input content
	var content string
	var err error

	if inputFile == "stdout" {
		// Special case: read from testscript's stdout buffer
		content = ts.Getenv("stdout")
		if content == "" {
			// Try to read stdout content using testscript's internal mechanism
			// This is a workaround since testscript doesn't expose stdout directly
			ts.Fatalf("txtproc: no stdout content available. Make sure to run 'exec' command before using txtproc with stdout")
		}
	} else if inputFile == "stderr" {
		// Special case: read from testscript's stderr buffer
		content = ts.Getenv("stderr")
		if content == "" {
			ts.Fatalf("txtproc: no stderr content available. Make sure to run 'exec' command before using txtproc with stderr")
		}
	} else {
		// Regular file
		inputPath := ts.MkAbs(inputFile)
		// #nosec G304 - File path is validated by testscript framework
		contentBytes, readErr := os.ReadFile(inputPath)
		if readErr != nil {
			ts.Fatalf("txtproc: failed to read %s: %v", inputFile, readErr)
		}
		content = string(contentBytes)
	}

	// Process content
	result, err := processText(content, operation, pattern, replacement)
	if err != nil {
		ts.Fatalf("txtproc: %v", err)
	}

	// Write output file
	outputPath := ts.MkAbs(outputFile)
	// Use more restrictive file permissions for security
	err = os.WriteFile(outputPath, []byte(result), 0600)
	if err != nil {
		ts.Fatalf("txtproc: failed to write %s: %v", outputFile, err)
	}
}

func processText(content, operation, pattern, replacement string) (string, error) {
	switch operation {
	case opReplace:
		return strings.ReplaceAll(content, pattern, replacement), nil

	case opReplaceRegex:
		re, err := regexp.Compile("(?m)" + pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex %q: %v", pattern, err)
		}
		return re.ReplaceAllString(content, replacement), nil

	case opRemoveLines:
		lines := strings.Split(content, "\n")
		var result []string
		for _, line := range lines {
			if !strings.Contains(line, pattern) {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n"), nil

	case opRemoveRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex %q: %v", pattern, err)
		}
		lines := strings.Split(content, "\n")
		var result []string
		for _, line := range lines {
			if !re.MatchString(line) {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n"), nil

	case opExtractLines:
		lines := strings.Split(content, "\n")
		var result []string
		for _, line := range lines {
			if strings.Contains(line, pattern) {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n"), nil

	case opExtractRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex %q: %v", pattern, err)
		}
		lines := strings.Split(content, "\n")
		var result []string
		for _, line := range lines {
			if re.MatchString(line) {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n"), nil

	default:
		return "", fmt.Errorf("unknown operation %q", operation)
	}
}
