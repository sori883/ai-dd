package okf

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func boundWarnings(warnings []Warning) []Warning {
	if warningSize(warnings) <= 6144 {
		return append([]Warning{}, warnings...)
	}
	kept := []Warning{}
	for i, w := range warnings {
		candidate := append(append([]Warning{}, kept...), w)
		if omitted := len(warnings) - i - 1; omitted > 0 {
			candidate = append(candidate, warningSummary(omitted))
		}
		if warningSize(candidate) > 6144 {
			break
		}
		kept = append(kept, w)
	}
	return append(kept, warningSummary(len(warnings)-len(kept)))
}

func warningSummary(count int) Warning {
	return Warning{Path: "", Reason: fmt.Sprintf("%d additional concept warnings omitted", count)}
}

func warningSize(warnings []Warning) int {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	// Encoding a slice of strings into bytes.Buffer cannot fail.
	if err := encoder.Encode(warnings); err != nil {
		return 6145
	}
	return b.Len() - 1
}
