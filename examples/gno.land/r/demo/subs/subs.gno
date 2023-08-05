package subs

import (
	"std"
	"strings"
	"time"

	"gno.land/p/demo/subscription"
	"gno.land/p/demo/ufmt"
)

const (
	SubsCost          = "10000ugnot"
	SubsExpiration    = time.Hour
	SubsMaxExpiration = SubsExpiration * 5
)

var (
	subs  *subscription.RecurringSubscription
	admin std.Address
)

func Subscribe() {
	now := time.Now()
	caller := std.GetOrigCaller()
	send := std.GetOrigSend()

	exp, valid := subs.Check(caller)
	switch {
	case !valid:
		// subscribe or renew subscription
		subs.SubscribeOrRenew(caller, send[0])
	case exp.Add(SubsExpiration).Before(now.Add(SubsMaxExpiration)):
		// expand current subscription
		subs.Expand(caller, send[0])
	default:
		panic("you have reached max subscribe expansion")
	}
}

func Render(path string) string {
	caller := std.GetOrigCaller()

	b := new(strings.Builder)

	b.WriteString("# Subscribe Board\n")
	b.WriteString("_CURRENT TIME: " + time.Now().Format("02-Jan-2006 15:04:05") + "_ \n\n")
	b.WriteString("_Subscribe cost is: " + SubsCost + " for 1 hour_\n\n")
	b.WriteString("## current subscription:\n\n")
	subs.Iterate("", "", func(sub std.Address, exp time.Time, valid bool) bool {
		if valid {
			b.WriteString("* " + sub.String() + " expire on " + exp.Format("02-Jan-2006 15:04:05") + "\n\n")
		}
	})

	return b.String()
}

func init() {
	cost := std.Coin{Amount: 10000, Denom: "ugnot"}
	admin = std.GetOrigCaller()
	subs = subscription.NewRecurringSubscription(SubsExpiration, cost)
}
