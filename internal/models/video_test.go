package models

import "testing"

func TestValidVisibility(t *testing.T) {
	if !ValidVisibility(VisibilityPublic) || !ValidVisibility(VisibilityUnlisted) || !ValidVisibility(VisibilityPrivate) {
		t.Fatal("known visibilities should be valid")
	}
	if ValidVisibility("draft") || ValidVisibility("") {
		t.Fatal("unknown / empty should be invalid")
	}
}

func TestVideo_EffectiveVisibility(t *testing.T) {
	if v := (&Video{}).EffectiveVisibility(); v != VisibilityPublic {
		t.Fatalf("empty -> public, got %q", v)
	}
	if v := (&Video{Visibility: VisibilityPrivate}).EffectiveVisibility(); v != VisibilityPrivate {
		t.Fatalf("got %q", v)
	}
}
