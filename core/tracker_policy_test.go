package core

import "testing"

func TestTrackerPolicyRequiresRevalidationOnVersionChange(t *testing.T) {
	status := SingBoxTrackerRevalidationStatus("v99.0.0")
	if !status.Required {
		t.Fatal("unexpectedly accepted unvalidated sing-box version")
	}
}
