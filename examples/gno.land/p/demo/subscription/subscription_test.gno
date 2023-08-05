package subscription

import (
	"std"
	"testing"
	"time"

	"gno.land/p/demo/testutils"
)

func TestNewRecurringSubscription(t *testing.T) {
	duration := time.Hour * 24 * 30
	amount := std.Coin{Amount: 1000, Denom: "ugnot"}

	subs := NewRecurringSubscription(duration, amount)
	if subs == nil {
		t.Error("Expected non-nil subscription, got nil")
	}
}

func TestSubscribeOrRenew(t *testing.T) {
	duration := time.Hour * 24 * 30
	amount := std.Coin{Amount: 1000, Denom: "ugnot"}
	subs := NewRecurringSubscription(duration, amount)
	alice := testutils.TestAddress("alice")

	// Test valid subscription
	deadline := subs.SubscribeOrRenew(alice, amount)
	if time.Now().After(deadline) || time.Now().Add(duration).Before(deadline) {
		t.Errorf("Expected deadline to be within %v, got %v", duration, deadline)
	}

	// Test invalid amount
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic due to invalid amount")
		}
	}()
	invalidAmount := std.Coin{Amount: 500, Denom: "ugnot"}
	subs.SubscribeOrRenew(alice, invalidAmount)
}

func TestCheck(t *testing.T) {
	duration := time.Hour * 24 * 30
	amount := std.Coin{Amount: 1000, Denom: "ugnot"}
	subs := NewRecurringSubscription(duration, amount)
	alice := testutils.TestAddress("alice")

	// Check for non-existent subscription
	_, valid := subs.Check(alice)
	if valid {
		t.Error("Expected invalid subscription, got valid")
	}

	// Check for valid subscription
	subs.SubscribeOrRenew(alice, amount)
	_, valid = subs.Check(alice)
	if !valid {
		t.Error("Expected valid subscription, got invalid")
	}

	// Check for expired subscription
	subs.Remove(alice)
	_, valid = subs.Check(alice)
	if valid {
		t.Error("Expected invalid subscription after invalidation, got valid")
	}
}
