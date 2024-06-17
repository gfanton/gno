package session

import (
	"net/http"
	"sync/atomic"

	"github.com/gnolang/gno/gnovm/stdlibs/strconv"
)

type MiddlewareOpts func(*Middleware)

func NewMiddleware(next http.Handler, opts ...MiddlewareOpts) http.Handler {
	mw := Middleware{
		Next:     next,
		Secure:   true,
		HTTPOnly: true,
	}
	for _, opt := range opts {
		opt(&mw)
	}
	return mw
}

func WithSecure(secure bool) MiddlewareOpts {
	return func(m *Middleware) {
		m.Secure = secure
	}
}

func WithHTTPOnly(httpOnly bool) MiddlewareOpts {
	return func(m *Middleware) {
		m.HTTPOnly = httpOnly
	}
}

type Middleware struct {
	Next     http.Handler
	Secure   bool
	HTTPOnly bool
}

func ID(r *http.Request) (id string) {
	cookie, err := r.Cookie("sessionID")
	if err != nil {
		return
	}

	return cookie.Value
}

var uid uint64

func (mw Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := ID(r)
	if id == "" {
		id = strconv.Itoa(int(atomic.AddUint64(&uid, 1)))
		http.SetCookie(w, &http.Cookie{
			Name:     "sessionID",
			Value:    id,
			Secure:   mw.Secure,
			HttpOnly: mw.HTTPOnly,
		})
	}
	mw.Next.ServeHTTP(w, r)
}
