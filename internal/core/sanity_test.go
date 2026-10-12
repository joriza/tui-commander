package core

import "testing"

const sanityConst = 42

func TestSanity(t *testing.T) {
	if sanityConst != 42 {
		t.Fatalf("expected 42, got %d", sanityConst)
	}
}
