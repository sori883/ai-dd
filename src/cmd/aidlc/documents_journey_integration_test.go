//go:build integration

package main

import "testing"

// TestIntentDocumentsJourney uses the actual CLI, document replacement, review,
// real RED/GREEN test processes and all four stage transitions in a fresh project.
func TestIntentDocumentsJourney(t *testing.T) { runBoundaryJourney(t) }
