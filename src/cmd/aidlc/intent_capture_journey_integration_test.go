//go:build integration

package main

import (
	"testing"
)

func TestCodexIntentCaptureFreshApprove(t *testing.T) {
	runCodexIntentCaptureFreshApprove(t)
}

func TestCodexIntentCaptureRejectReviseApprove(t *testing.T) {
	runCodexIntentCaptureRejectReviseApprove(t)
}

func TestCodexIntentCaptureFailsClosed(t *testing.T) {
	runCodexIntentCaptureFailsClosed(t)
}
