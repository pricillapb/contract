package netutil

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
)

type IPTracker struct {
	window, contactWindow time.Duration
	minStatements         int
	clock                 clock

	mu         sync.Mutex
	statements map[string]ipStatement
	contact    map[string]time.Time
}

type ipStatement struct {
	endpoint string
	time     time.Time
}

type clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

func NewIPTracker(window, contactWindow time.Duration, minStatements int) *IPTracker {
	return &IPTracker{
		window:        window,
		contactWindow: contactWindow,
		statements:    make(map[string]ipStatement),
		minStatements: minStatements,
		contact:       make(map[string]time.Time),
		clock:         realClock{},
	}
}

func (it *IPTracker) PredictFullConeNAT(t mclock.AbsTime) bool {
	it.mu.Lock()
	defer it.mu.Unlock()

	now := it.clock.Now()
	it.gcContact(now)
	it.gcStatements(now)
	for host := range it.statements {
		if _, ok := it.contact[host]; !ok {
			return true
		}
	}
	return false
}

func (it *IPTracker) PredictEndpoint() string {
	it.mu.Lock()
	defer it.mu.Unlock()

	it.gcStatements(it.clock.Now())

	// Find IP with most statements.
	counts := make(map[string]int)
	maxcount, max := 0, ""
	for _, s := range it.statements {
		c := counts[s.endpoint] + 1
		counts[s.endpoint] = c
		if c > maxcount && c > it.minStatements {
			maxcount, max = c, s.endpoint
		}
	}
	return max
}

func (it *IPTracker) AddStatement(host, endpoint string) {
	it.mu.Lock()
	defer it.mu.Unlock()

	it.statements[host] = ipStatement{endpoint, it.clock.Now()}
}

func (it *IPTracker) AddContact(host string) {
	it.mu.Lock()
	defer it.mu.Unlock()

	it.contact[host] = it.clock.Now()
}

func (it *IPTracker) gcStatements(now time.Time) {
	cutoff := now.Add(-it.window)
	for host, s := range it.statements {
		if s.time.Before(cutoff) {
			delete(it.statements, host)
		}
	}
}

func (it *IPTracker) gcContact(now time.Time) {
	cutoff := now.Add(-it.contactWindow)
	for host, ct := range it.contact {
		if ct.Before(cutoff) {
			delete(it.contact, host)
		}
	}
}
