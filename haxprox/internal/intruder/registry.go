package intruder

import (
	"context"
	"fmt"
	"sync"
)

// AttackRun represents one in-progress or completed attack, tracked so
// the UI can poll its results and optionally stop it early.
type AttackRun struct {
	ID     int64
	cancel context.CancelFunc
	mu     sync.Mutex
	results []Result
	done   bool
}

func (r *AttackRun) appendResult(res Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, res)
}

func (r *AttackRun) Results() ([]Result, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Result, len(r.results))
	copy(out, r.results)
	return out, r.done
}

// Registry tracks all AttackRuns by ID - same mutex-protected-map shape
// as intercept.Queue's pending map.
type Registry struct {
	mu     sync.Mutex
	nextID int64
	runs   map[int64]*AttackRun
}

func NewRegistry() *Registry {
	return &Registry{runs: make(map[int64]*AttackRun)}
}

// Start kicks off a new attack in the background and returns its
// AttackRun handle immediately - results accumulate as they come in and
// are read via AttackRun.Results().
func (reg *Registry) Start(attack Attack) *AttackRun {
	reg.mu.Lock()
	reg.nextID++
	id := reg.nextID
	ctx, cancel := context.WithCancel(context.Background())
	run := &AttackRun{ID: id, cancel: cancel}
	reg.runs[id] = run
	reg.mu.Unlock()

	resultChan := make(chan Result)
	go Run(ctx, attack, resultChan) // Run is the package-level function in attack.go
	go func() {
		for res := range resultChan {
			run.appendResult(res)
		}
		run.mu.Lock()
		run.done = true
		run.mu.Unlock()
	}()

	return run
}

func (reg *Registry) Get(id int64) (*AttackRun, error) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	run, ok := reg.runs[id]
	if !ok {
		return nil, fmt.Errorf("intruder: no run with id %d", id)
	}
	return run, nil
}

func (reg *Registry) Stop(id int64) error {
	run, err := reg.Get(id)
	if err != nil {
		return err
	}
	run.cancel()
	return nil
}