// Package subscription provides a basic Gno library for managing recurring
// subscriptions.
// The library allows users to create and manage different subscription models
// to control access to specific features or resources.
//
// Example Usage:
//
//	import "gno.land/p/demo/subscription"
//
//	// Duration of 30 days and amount of 1000 units subscription.New
//	var subs = subscription.NewRecurringSubscription(time.Hour*24*30, 1000)
//
//	func SomeHandler() {
//		subs.CheckOrigCaller()
//
//		// Proceed with handling the request that requires a subscription
//		// ...
//	}
package subscription // import "gno.land/p/demo/subscription"

import (
	"std"
	"strconv"
	"time"

	"gno.land/p/demo/avl"
)

// RecurringSubscription represents a recurring subscription model with a given
// duration and amount.
type RecurringSubscription struct {
	duration time.Duration
	amount   std.Coin
	subs     *avl.Tree // std.Address -> time.Time
}

// NewRecurringSubscription creates a new instance of RecurringSubscription with
// the specified duration and amount.
//
// The subs parameter is an AVL tree that holds the subscription status for each
// user (std.Address) with their expiration time (time.Time).
func NewRecurringSubscription(duration time.Duration, amount std.Coin) *RecurringSubscription {
	return &RecurringSubscription{
		duration: duration,
		amount:   amount,
		subs:     avl.NewTree(),
	}
}

// Check verifies the subscription status for the specified address.
// It panics if the subscription does not exist or has expired.
//
// Returns the expiration time if valid
func (rs *RecurringSubscription) Check(sub std.Address) (exp time.Time, valid bool) {
	now := time.Now()

	exp, valid = rs.getAddress(sub)
	if !valid {
		return
	}

	if now.Before(exp) {
		valid = true
	}

	return
}

func (rs *RecurringSubscription) CheckOrigCaller() (exp time.Time, valid bool) {
	caller := std.GetOrigCaller()
	return rs.Check(caller)
}

// SubscribeOrRenew handles both new subscriptions and renewals for a given address.
// It panics if the subscription is active or if the provided amount does not
// match the required amount.
//
// Returns the expiration deadline for the subscription.
func (rs *RecurringSubscription) SubscribeOrRenew(sub std.Address, coin std.Coin) (deadline time.Time) {
	now := time.Now()

	exp, exist := rs.getAddress(sub)
	if exist && now.Before(exp) {
		panic("subscription is active")
	}

	// XXX: should be cool if we can use IsEqual here
	if coin.Amount != rs.amount.Amount || coin.Denom != rs.amount.Denom {
		panic("invalid amount for subscription, need exactly: " +
			strconv.Itoa(int(rs.amount.Amount)) + rs.amount.Denom)
	}

	deadline = now.Add(rs.duration)
	rs.setAddress(sub, deadline)
	return deadline
}

func (rs *RecurringSubscription) SubscribeOrRenewOrigCaller() (exp time.Time) {
	caller := std.GetOrigCaller()
	send := std.GetOrigSend()

	return rs.SubscribeOrRenew(caller, send[0])
}

// Expand an existing subscription for a given address by the duration defined
// in the RecurringSubscription.
// It validates that the subscription is active and that the provided amount
// exactly matches the recurring subscription amount.
//
// Returns the new expiration deadline for the expanded subscription, and panics
// if the subscription is not active or if the amount is invalid.
func (rs *RecurringSubscription) Expand(sub std.Address, coin std.Coin) (deadline time.Time) {
	exp, valid := rs.Check(sub)
	if !valid {
		panic("subscription is not active")
	}

	// TODO: should be cool if we can use IsEqual here
	if !coin.IsEqual(rs.amount) {
		panic("invalid amount for subscription expansion, need exactly: " +
			strconv.Itoa(int(rs.amount.Amount)) + rs.amount.Denom)
	}

	deadline = exp.Add(rs.duration)
	rs.setAddress(sub, deadline)
	return deadline
}

func (rs *RecurringSubscription) ExpandOrigCaller() (exp time.Time) {
	caller := std.GetOrigCaller()
	send := std.GetOrigSend()

	return rs.Expand(caller, send[0])
}

// Remove deletes the subscription for the given address from the subscription list.
func (rs *RecurringSubscription) Remove(sub std.Address) {
	rs.subs.Remove(sub.String())
}

type IterSubCb func(sub std.Address, exp time.Time, valid bool) bool

func (rs *RecurringSubscription) Iterate(start, end string, cb IterSubCb) bool {
	now := time.Now()
	return rs.subs.Iterate(start, end, func(key string, value interface{}) bool {
		exp := value.(time.Time)
		return cb(std.Address(key), exp, now.Before(exp))
	})
}

// TODO: add other iterates methods ?

func (rs *RecurringSubscription) getAddress(sub std.Address) (exp time.Time, exist bool) {
	var value interface{}
	if value, exist = rs.subs.Get(sub.String()); exist {
		exp = value.(time.Time)
	}

	return exp, exist
}

func (rs *RecurringSubscription) setAddress(sub std.Address, exp time.Time) {
	rs.subs.Set(sub.String(), exp)
}
