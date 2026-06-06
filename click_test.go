package menuet

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Application is a singleton, so these tests must mutate App().Clicked in
// sequence and reset it on the way out.

func TestHasTopLevelClickedReflectsClickedField(t *testing.T) {
	a := App()
	defer func() { a.Clicked = nil }()

	a.Clicked = nil
	if hasTopLevelClicked() {
		t.Errorf("hasTopLevelClicked() with nil callback = true, want false")
	}

	a.Clicked = func() {}
	if !hasTopLevelClicked() {
		t.Errorf("hasTopLevelClicked() with set callback = false, want true")
	}

	a.Clicked = nil
	if hasTopLevelClicked() {
		t.Errorf("hasTopLevelClicked() after clearing = true, want false")
	}
}

func TestTopLevelClickedInvokesCallback(t *testing.T) {
	a := App()
	defer func() { a.Clicked = nil }()

	var called int32
	var wg sync.WaitGroup
	wg.Add(1)
	a.Clicked = func() {
		atomic.StoreInt32(&called, 1)
		wg.Done()
	}

	topLevelClicked()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("callback was not invoked within 1s")
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("callback didn't run")
	}
}

func TestTopLevelClickedIsNoOpWhenNil(t *testing.T) {
	a := App()
	defer func() { a.Clicked = nil }()

	a.Clicked = nil
	// Should not panic.
	topLevelClicked()
}
