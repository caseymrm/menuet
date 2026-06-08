package menuet

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBuildInternalItemSearchSerialization(t *testing.T) {
	// Search uses Text as the placeholder on the ObjC side; Type must be
	// "search" so menuet.m's populate: dispatches to the search-view path.
	item := buildInternalItem(
		Search{Placeholder: "Search apps…", Results: func(string) []MenuItem { return nil }},
		"unique-1", "parent-1",
	)
	if item.Type != "search" {
		t.Errorf("Type = %q, want %q", item.Type, "search")
	}
	if item.Text != "Search apps…" {
		t.Errorf("Text = %q, want placeholder copied from Search.Placeholder", item.Text)
	}

	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"Type":"search"`,
		`"Text":"Search apps…"`,
		`"Unique":"unique-1"`,
		`"ParentUnique":"parent-1"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON missing %q\nfull: %s", want, got)
		}
	}
}

func TestApplicationSearchResultsRunsCallbackWithQuery(t *testing.T) {
	a := App()
	defer func() {
		a.visibleMenuItemsMutex.Lock()
		a.visibleMenuItems = make(map[string]internalItem)
		a.visibleMenuItemsMutex.Unlock()
	}()

	var received atomic.Value
	received.Store("")
	resultsFn := func(q string) []MenuItem {
		received.Store(q)
		return []MenuItem{Regular{Text: "Result for " + q}}
	}

	const searchUnique = "search-1"
	a.visibleMenuItemsMutex.Lock()
	a.visibleMenuItems[searchUnique] = internalItem{
		Unique: searchUnique,
		Type:   "search",
		item:   Search{Placeholder: "go", Results: resultsFn},
	}
	a.visibleMenuItemsMutex.Unlock()

	got := a.searchResults(searchUnique, "hello")
	if received.Load().(string) != "hello" {
		t.Errorf("Search.Results received %q, want %q", received.Load(), "hello")
	}
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}
	if got[0].Text != "Result for hello" {
		t.Errorf("Text = %q, want %q", got[0].Text, "Result for hello")
	}
	if got[0].ParentUnique != searchUnique {
		t.Errorf("ParentUnique = %q, want %q", got[0].ParentUnique, searchUnique)
	}
}

func TestApplicationSearchResultsCleansUpPriorResults(t *testing.T) {
	// Each keystroke replaces the prior result items. Previously registered
	// results (with ParentUnique == searchUnique) must be removed from
	// visibleMenuItems so the map doesn't grow unbounded.
	a := App()
	defer func() {
		a.visibleMenuItemsMutex.Lock()
		a.visibleMenuItems = make(map[string]internalItem)
		a.visibleMenuItemsMutex.Unlock()
	}()

	const searchUnique = "search-2"
	a.visibleMenuItemsMutex.Lock()
	a.visibleMenuItems[searchUnique] = internalItem{
		Unique: searchUnique,
		Type:   "search",
		item: Search{Results: func(q string) []MenuItem {
			return []MenuItem{Regular{Text: "first"}, Regular{Text: "second"}}
		}},
	}
	a.visibleMenuItemsMutex.Unlock()

	a.searchResults(searchUnique, "q1")
	// Count how many items have this search as their parent.
	count := func() int {
		a.visibleMenuItemsMutex.RLock()
		defer a.visibleMenuItemsMutex.RUnlock()
		n := 0
		for _, v := range a.visibleMenuItems {
			if v.ParentUnique == searchUnique {
				n++
			}
		}
		return n
	}
	if got := count(); got != 2 {
		t.Errorf("after first query: %d result items, want 2", got)
	}

	a.searchResults(searchUnique, "q2")
	if got := count(); got != 2 {
		t.Errorf("after second query: %d result items, want 2 (old ones should have been replaced, not appended)", got)
	}
}

func TestApplicationSearchResultsMissingItemReturnsNil(t *testing.T) {
	a := App()
	if got := a.searchResults("does-not-exist", ""); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestApplicationSearchResultsWrongTypeReturnsNil(t *testing.T) {
	// If something registered under a unique isn't actually a Search, we
	// should log and return nil rather than panic on a bad type assertion.
	a := App()
	defer func() {
		a.visibleMenuItemsMutex.Lock()
		a.visibleMenuItems = make(map[string]internalItem)
		a.visibleMenuItemsMutex.Unlock()
	}()
	a.visibleMenuItemsMutex.Lock()
	a.visibleMenuItems["regular-1"] = internalItem{
		Unique: "regular-1",
		item:   Regular{Text: "not a search"},
	}
	a.visibleMenuItemsMutex.Unlock()

	if got := a.searchResults("regular-1", ""); got != nil {
		t.Errorf("expected nil for non-Search item, got %v", got)
	}
}

func TestApplicationSearchResultsNilCallbackReturnsNil(t *testing.T) {
	a := App()
	defer func() {
		a.visibleMenuItemsMutex.Lock()
		a.visibleMenuItems = make(map[string]internalItem)
		a.visibleMenuItemsMutex.Unlock()
	}()
	a.visibleMenuItemsMutex.Lock()
	a.visibleMenuItems["search-3"] = internalItem{
		Unique: "search-3",
		item:   Search{Placeholder: "x"}, // no Results
	}
	a.visibleMenuItemsMutex.Unlock()

	if got := a.searchResults("search-3", "anything"); got != nil {
		t.Errorf("expected nil when Search.Results is nil, got %v", got)
	}
}

func TestApplicationSearchResultsConcurrentCallsDontDeadlock(t *testing.T) {
	// searchResults takes the write lock when mutating visibleMenuItems.
	// Make sure rapid keystrokes from the ObjC side don't deadlock against
	// concurrent children() lookups.
	a := App()
	defer func() {
		a.visibleMenuItemsMutex.Lock()
		a.visibleMenuItems = make(map[string]internalItem)
		a.visibleMenuItemsMutex.Unlock()
	}()
	a.visibleMenuItemsMutex.Lock()
	a.visibleMenuItems["search-4"] = internalItem{
		Unique: "search-4",
		item:   Search{Results: func(q string) []MenuItem { return []MenuItem{Regular{Text: q}} }},
	}
	a.visibleMenuItemsMutex.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a.searchResults("search-4", string(rune('a'+i%26)))
		}(i)
	}
	wg.Wait()
}
