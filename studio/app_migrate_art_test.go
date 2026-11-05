package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"tcgstudio/internal/library"
)

// fakeConverter counts conversions per folder+set and fails if two copies of one set are converted at the same time.
type fakeConverter struct {
	mu      sync.Mutex
	calls   map[string]int
	busy    map[string]bool
	fail    map[string]bool // folder|id → error
	overlap bool
}

func (f *fakeConverter) convert(dir, id string, kinds []string) (bool, error) {
	f.mu.Lock()
	if f.busy[id] {
		f.overlap = true
	}
	f.busy[id] = true
	f.calls[dir+"|"+id]++
	f.mu.Unlock()
	time.Sleep(5 * time.Millisecond) // let other workers run meanwhile
	f.mu.Lock()
	f.busy[id] = false
	fail := f.fail[dir+"|"+id]
	f.mu.Unlock()
	if fail {
		return false, errors.New("broken set")
	}
	return true, nil
}

// TestRunArtJobs: every listed set converts once per folder, copies of one set never at the same time, and a failure is
// reported without stopping the others.
func TestRunArtJobs(t *testing.T) {
	job := func(dir string, ids ...string) artJob {
		j := artJob{name: dir, dir: dir, sets: map[string][]string{}, order: ids}
		for _, id := range ids {
			j.sets[id] = []string{"Box"}
		}
		return j
	}
	f := &fakeConverter{calls: map[string]int{}, busy: map[string]bool{}, fail: map[string]bool{"catalog|z": true}}
	_, err := runArtJobs(context.Background(), []artJob{job("setupA", "x"), job("catalog", "x", "z")}, f.convert, func(library.Progress) {})
	if err == nil {
		t.Fatal("a failed set should be reported")
	}
	if f.overlap {
		t.Error("two copies of one set were converted at the same time")
	}
	for _, k := range []string{"setupA|x", "catalog|x", "catalog|z"} {
		if f.calls[k] != 1 {
			t.Errorf("%s converted %d times, want once", k, f.calls[k])
		}
	}
}
