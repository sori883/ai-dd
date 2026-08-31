package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sori883/ai-dd/src/internal/workspace"
)

type spaceListRow struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func runSpaceList(
	explicitDir string,
	jsonOutput bool,
	stdout io.Writer,
	stderr io.Writer,
	listSpaces func(string) ([]workspace.Space, error),
) int {
	spaces, err := listSpaces(explicitDir)
	if err != nil {
		return writeSpaceError(stderr, err)
	}
	var output strings.Builder
	if jsonOutput {
		listing := struct {
			Active string         `json:"active"`
			Spaces []spaceListRow `json:"spaces"`
		}{Active: "default", Spaces: make([]spaceListRow, 0, len(spaces))}
		activeFound := false
		for _, space := range spaces {
			listing.Spaces = append(listing.Spaces, spaceListRow{Name: space.Name, Active: space.Active})
			if space.Active && !activeFound {
				listing.Active = space.Name
				activeFound = true
			}
		}
		if err := json.NewEncoder(&output).Encode(listing); err != nil {
			return writeSpaceError(stderr, fmt.Errorf("encode spaces: %w", err))
		}
	} else {
		output.WriteString("Spaces:\n")
		for _, space := range spaces {
			marker := " "
			if space.Active {
				marker = "*"
			}
			fmt.Fprintf(
				&output,
				"%s %s\n",
				marker,
				space.Name,
			)
		}
	}
	n, err := io.WriteString(stdout, output.String())
	if err == nil && n != output.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		return writeSpaceError(stderr, fmt.Errorf("write stdout: %w", err))
	}
	return 0
}
