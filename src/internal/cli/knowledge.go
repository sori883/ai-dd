package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sori883/ai-dd/src/internal/okf"
)

func runKnowledgeSearch(args []string, stdout, stderr io.Writer, dependencies Dependencies) int {
	if dependencies.PrepareOutput != nil {
		dependencies.PrepareOutput()
	}
	options, dir, err := knowledgeArguments(args)
	if err != nil {
		return writeDeliverySyntaxError(stderr, err)
	}
	if dependencies.SearchKnowledge == nil {
		return knowledgeError(stderr, errors.New("knowledge search callback is unavailable"))
	}
	result, err := dependencies.SearchKnowledge(options, dir)
	if err != nil {
		return knowledgeError(stderr, err)
	}
	if result.Results == nil {
		result.Results = []okf.Result{}
	}
	if result.Warnings == nil {
		result.Warnings = []okf.Warning{}
	}
	for i := range result.Results {
		if result.Results[i].Tags == nil {
			result.Results[i].Tags = []string{}
		}
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		return knowledgeError(stderr, err)
	}
	n, err := stdout.Write(buffer.Bytes())
	if err == nil && n != buffer.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		return knowledgeError(stderr, fmt.Errorf("write stdout: %w", err))
	}
	return 0
}

func knowledgeArguments(args []string) (okf.SearchOptions, string, error) {
	options := okf.SearchOptions{Limit: 4}
	if len(args) < 2 || args[1] != "search" {
		return options, "", errors.New("knowledge requires search")
	}
	dir := ""
	seen := map[string]bool{}
	for i := 2; i < len(args); i++ {
		key, value, hasEquals := strings.Cut(args[i], "=")
		switch key {
		case "--tag", "--type", "--query", "--limit", "--project-dir":
		default:
			return options, "", fmt.Errorf("unknown knowledge search argument %q", args[i])
		}
		if key != "--tag" && key != "--type" && seen[key] {
			return options, "", fmt.Errorf("duplicate %s", key)
		}
		seen[key] = true
		if !hasEquals {
			if i+1 == len(args) || strings.HasPrefix(args[i+1], "-") {
				return options, "", fmt.Errorf("%s requires a value", key)
			}
			i++
			value = args[i]
		}
		if value == "" {
			return options, "", fmt.Errorf("%s requires a nonempty value", key)
		}
		switch key {
		case "--tag":
			options.Tags = append(options.Tags, value)
		case "--type":
			options.Types = append(options.Types, value)
		case "--query":
			options.Query = value
			options.HasQuery = true
		case "--project-dir":
			dir = value
		case "--limit":
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				return options, "", errors.New("limit must be between 1 and 100")
			}
			options.Limit = limit
		}
	}
	return options, dir, okf.ValidateSearch(options)
}

func knowledgeError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "aidlc: %v\n", err)
	return 1
}
