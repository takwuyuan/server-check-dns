package main

import (
	"fmt"
	"sync"

	"github.com/miekg/dns"
)

type CheckServeMux struct {
	dns.ServeMux
	z map[string]dns.Handler
	m *sync.RWMutex
}

var nameserver string

// NewServeMux allocates and returns a new ServeMux.
func NewCheckServeMux(forward string) *CheckServeMux {
	nameserver = forward
	return &CheckServeMux{z: make(map[string]dns.Handler), m: new(sync.RWMutex)}
}

// HandleFailed returns a HandlerFunc that returns SERVFAIL for every request it gets.
func HandleFailed(w dns.ResponseWriter, r *dns.Msg) {
	c := new(dns.Client)
	resp, rtt, err := c.Exchange(r, nameserver)

	// does not matter if this write fails
	if err != nil {
		fmt.Printf("failed %+v %+v, %+v\n", resp, rtt, err)
		resp = new(dns.Msg)
		resp.SetRcode(r, dns.RcodeServerFailure)
	}

	w.WriteMsg(resp)
}

//
// オーバーライドするために必要な関数たち
//

func failedHandler() dns.Handler { return dns.HandlerFunc(HandleFailed) }

// Handle adds a handler to the ServeMux for pattern.
func (mux *CheckServeMux) Handle(pattern string, handler dns.Handler) {
	if pattern == "" {
		panic("dns: invalid pattern " + pattern)
	}
	mux.m.Lock()
	mux.z[dns.Fqdn(pattern)] = handler
	mux.m.Unlock()
}

// HandleFunc adds a handler function to the ServeMux for pattern.
func (mux *CheckServeMux) HandleFunc(pattern string, handler func(dns.ResponseWriter, *dns.Msg)) {
	mux.Handle(pattern, dns.HandlerFunc(handler))
}

func (mux *CheckServeMux) ServeDNS(w dns.ResponseWriter, request *dns.Msg) {
	var h dns.Handler
	if len(request.Question) < 1 { // allow more than one question
		h = failedHandler()
	} else {
		if h = mux.match(request.Question[0].Name, request.Question[0].Qtype); h == nil {
			h = failedHandler()
		}
	}
	h.ServeDNS(w, request)
}

func (mux *CheckServeMux) match(q string, t uint16) dns.Handler {
	mux.m.RLock()
	defer mux.m.RUnlock()
	var handler dns.Handler
	b := make([]byte, len(q)) // worst case, one label of length q
	off := 0
	end := false
	for {
		l := len(q[off:])
		for i := 0; i < l; i++ {
			b[i] = q[off+i]
			if b[i] >= 'A' && b[i] <= 'Z' {
				b[i] |= ('a' - 'A')
			}
		}
		if h, ok := mux.z[string(b[:l])]; ok { // causes garbage, might want to change the map key
			if t != dns.TypeDS {
				return h
			}
			// Continue for DS to see if we have a parent too, if so delegeate to the parent
			handler = h
		}
		off, end = dns.NextLabel(q, off)
		if end {
			break
		}
	}
	// Wildcard match, if we have found nothing try the root zone as a last resort.
	if h, ok := mux.z["."]; ok {
		return h
	}
	return handler
}
