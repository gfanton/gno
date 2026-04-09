package probeauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
)

// ChallengeSize is the number of random bytes in a challenge.
const ChallengeSize = 32

// GenerateChallenge returns a cryptographically random challenge of ChallengeSize bytes.
func GenerateChallenge() ([]byte, error) {
	b := make([]byte, ChallengeSize)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate challenge: %w", err)
	}
	return b, nil
}

// SignChallenge signs the challenge with the given private key.
func SignChallenge(priv ed25519.PrivateKey, challenge []byte) []byte {
	return ed25519.Sign(priv, challenge)
}

// VerifyChallenge checks that signature is a valid ed25519 signature of challenge by pub.
func VerifyChallenge(pub ed25519.PublicKey, challenge, signature []byte) bool {
	return ed25519.Verify(pub, challenge, signature)
}
