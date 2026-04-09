package probeauth

import (
	"testing"
)

func TestGenerateChallenge(t *testing.T) {
	c, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c) != ChallengeSize {
		t.Fatalf("expected challenge length %d, got %d", ChallengeSize, len(c))
	}
}
