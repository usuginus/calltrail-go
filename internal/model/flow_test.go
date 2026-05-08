package model

import "testing"

func TestTrailAppendLayerCallKeepsConfiguredOrder(t *testing.T) {
	trail := NewTrail([]string{"application", "repository"})
	trail.AppendLayerCall("repository", CallRef{Symbol: "repository.Find"})
	trail.AppendLayerCall("application", CallRef{Symbol: "application.Run"})

	if len(trail.Layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(trail.Layers))
	}
	if got := trail.Layers[0].Name; got != "application" {
		t.Fatalf("first layer = %q, want application", got)
	}
	if got := trail.Layers[1].Name; got != "repository" {
		t.Fatalf("second layer = %q, want repository", got)
	}
}

func TestTrailAppendLayerCallAppendsUnknownOrderLayers(t *testing.T) {
	trail := NewTrail([]string{"application"})
	trail.AppendLayerCall("repository", CallRef{Symbol: "repository.Find"})
	trail.AppendLayerCall("application", CallRef{Symbol: "application.Run"})
	trail.AppendLayerCall("domain", CallRef{Symbol: "domain.Validate"})

	if len(trail.Layers) != 3 {
		t.Fatalf("layers = %d, want 3", len(trail.Layers))
	}
	if got := trail.Layers[0].Name; got != "application" {
		t.Fatalf("first layer = %q, want application", got)
	}
	if got := trail.Layers[1].Name; got != "repository" {
		t.Fatalf("second layer = %q, want repository", got)
	}
	if got := trail.Layers[2].Name; got != "domain" {
		t.Fatalf("third layer = %q, want domain", got)
	}
}

func TestBranchCaseAppendLayerCallKeepsConfiguredOrder(t *testing.T) {
	var branchCase BranchCase
	branchCase.AppendLayerCall("persistence", CallRef{Symbol: "store.Insert"}, []string{"domain", "persistence"})
	branchCase.AppendLayerCall("domain", CallRef{Symbol: "policy.Validate"}, []string{"domain", "persistence"})

	if len(branchCase.Layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(branchCase.Layers))
	}
	if got := branchCase.Layers[0].Name; got != "domain" {
		t.Fatalf("first layer = %q, want domain", got)
	}
	if got := branchCase.Layers[1].Name; got != "persistence" {
		t.Fatalf("second layer = %q, want persistence", got)
	}
}
