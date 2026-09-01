package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sori883/ai-dd/src/internal/workspace"
)

type intentListJSON struct {
	Active  *string             `json:"active"`
	Space   string              `json:"space"`
	Intents []intentListJSONRow `json:"intents"`
}

type intentListJSONRow struct {
	UUID    string   `json:"uuid"`
	Slug    string   `json:"slug"`
	Status  string   `json:"status"`
	Repos   []string `json:"repos"`
	DirName *string  `json:"dirName"`
	Active  bool     `json:"active"`
}

func runIntentList(
	explicitDir string,
	jsonOutput bool,
	stdout io.Writer,
	stderr io.Writer,
	listIntents func(string) (workspace.IntentListing, error),
) int {
	listing, err := listIntents(explicitDir)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	var output strings.Builder
	if jsonOutput {
		jsonListing := intentListJSON{
			Space:   listing.SpaceName,
			Intents: make([]intentListJSONRow, 0, len(listing.Intents)),
		}
		for _, intent := range listing.Intents {
			repos := intent.Repos
			if repos == nil {
				repos = []string{}
			}
			jsonListing.Intents = append(jsonListing.Intents, intentListJSONRow{
				UUID: intent.UUID, Slug: intent.Slug, Status: intent.Status, Repos: repos,
				DirName: intent.DirName, Active: intent.Active,
			})
			if intent.Active && intent.DirName != nil && jsonListing.Active == nil {
				active := *intent.DirName
				jsonListing.Active = &active
			}
		}
		if err := json.NewEncoder(&output).Encode(jsonListing); err != nil {
			return writeCommandError(stderr, fmt.Errorf("encode intents: %w", err))
		}
	} else {
		if len(listing.Intents) == 0 {
			fmt.Fprintf(
				&output,
				"No intents in space %q yet. Start one by describing what to build: /aidlc \"build the auth service\"\n",
				listing.SpaceName,
			)
		} else {
			fmt.Fprintf(&output, "Intents in space %q:\n", listing.SpaceName)
			hasActive := false
			for _, intent := range listing.Intents {
				marker := " "
				if intent.Active {
					marker = "*"
					hasActive = true
				}
				name := intent.Slug
				if intent.DirName != nil {
					name = *intent.DirName
				}
				fmt.Fprintf(&output, "%s %s  [%s]\n", marker, name, intent.Status)
			}
			if !hasActive {
				output.WriteString("\n(no active intent — switch with /aidlc intent <name>)\n")
			}
		}
	}
	n, err := io.WriteString(stdout, output.String())
	if err == nil && n != output.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		return writeCommandError(stderr, fmt.Errorf("write stdout: %w", err))
	}
	return 0
}
