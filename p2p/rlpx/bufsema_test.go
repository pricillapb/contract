package rlpx

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func TestBufSemaCountSimple(t *testing.T) {
	sem := newBufSema(2000)

	checkacquire := func(count, wantCount uint32, wantErr error) {
		err := sem.waitAcquire(count, 10*time.Millisecond)
		if !reflect.DeepEqual(err, wantErr) {
			t.Fatalf("wrong error after acquire(%d): got %q, want %q", count, err, wantErr)
		}
		if val := sem.get(); val != wantCount {
			t.Fatalf("wrong count after acquire(%d): got %d, want %d", count, val, wantCount)
		}
	}
	checkrelease := func(count, wantCount uint32) {
		sem.release(count)
		if val := sem.get(); val != wantCount {
			t.Fatalf("wrong count after release(%d): got %d, want %d", count, val, wantCount)
		}
	}

	// Check that the counter is maintained correctly.
	checkacquire(1000, 1000, nil)
	checkacquire(1000, 0, nil)
	checkacquire(1000, 0, errAcquireTimeout)
	checkrelease(900, 900)
	checkrelease(900, 1800)
	checkrelease(199, 1999)
	checkrelease(1, 2000)

	// Check that requesting more than sem.cap fails.
	checkacquire(2001, 2000, errors.New("requested amount 2001 exceeds semaphore cap of 2000"))

	// Check that a failed waitAcquire leaves sem.val as is when it is < sem.cap.
	checkacquire(500, 1500, nil)
	checkrelease(200, 1700)
	checkacquire(2000, 1700, errAcquireTimeout)
}

// This test checks that release wakes up waitAcquire.
func TestBufSemaRace(t *testing.T) {
	const (
		waitCount  = 10000
		iterations = 5000
	)
	sem := newBufSema(waitCount)
	pleaserelease := make(chan uint32, 500)
	releaser := func() {
		for rv := range pleaserelease {
			sem.release(rv)
		}
	}
	defer close(pleaserelease)
	go releaser()
	go releaser()
	go releaser()

	for i := 0; i < iterations; i++ {
		if err := sem.waitAcquire(waitCount, 1*time.Second); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		for i := uint32(0); i < waitCount; {
			rv := rand.Uint32() % waitCount
			if i+rv > waitCount {
				rv = waitCount - i
			}
			i += rv
			pleaserelease <- rv
		}
	}
}
