package core

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// counts the jobs-changed actions an app announces.
type job_observer struct {
	mu    sync.Mutex
	count int
}

func (o *job_observer) OnResultsChanged(_, _ *Snapshot) {}
func (o *job_observer) OnAction(action Action) {
	if action.Type == ACTION_JOBS_CHANGED {
		o.mu.Lock()
		o.count++
		o.mu.Unlock()
	}
}

func TestJobLifecycle(t *testing.T) {
	app := NewApp()
	go app.ProcessUpdateLoop()
	defer app.Stop()

	job := app.StartJob("checking for updates", 10)
	job.Tick(4)

	actual := app.Jobs()
	assert.Len(t, actual, 1)
	assert.Equal(t, "checking for updates", actual[0].Name)
	assert.Equal(t, 4, actual[0].Done)
	assert.Equal(t, 10, actual[0].Total)

	job.SetTotal(12)
	assert.Equal(t, 12, app.Jobs()[0].Total)

	job.Finish()
	assert.Empty(t, app.Jobs())

	// finished jobs ignore further calls
	job.Tick(1)
	job.Finish()
	assert.Empty(t, app.Jobs())
}

func TestJobs__concurrent_and_ordered(t *testing.T) {
	app := NewApp()
	go app.ProcessUpdateLoop()
	defer app.Stop()

	first := app.StartJob("first", 0)
	time.Sleep(time.Millisecond)
	second := app.StartJob("second", 0)

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() { first.Tick(1) })
		wg.Go(func() { second.Tick(2) })
	}
	wg.Wait()

	actual := app.Jobs()
	assert.Equal(t, []string{"first", "second"}, []string{actual[0].Name, actual[1].Name})
	assert.Equal(t, 50, actual[0].Done)
	assert.Equal(t, 100, actual[1].Done)

	first.Finish()
	second.Finish()
	app.WaitForJobs()
	assert.Empty(t, app.Jobs())
}

func TestJobs__deferred_finish_on_error(t *testing.T) {
	app := NewApp()
	go app.ProcessUpdateLoop()
	defer app.Stop()

	work := func() error {
		job := app.StartJob("failing", 3)
		defer job.Finish()
		job.Tick(1)
		return errors.New("host unreachable")
	}
	assert.Error(t, work())
	assert.Empty(t, app.Jobs())
}

func TestJobs__observers_told(t *testing.T) {
	app := NewApp()
	obs := &job_observer{}
	app.AddObserver(obs)
	go app.ProcessUpdateLoop()
	defer app.Stop()

	job := app.StartJob("x", 1)
	job.Tick(1)
	job.Finish()

	// announcements are queued asynchronously
	assert.Eventually(t, func() bool {
		obs.mu.Lock()
		defer obs.mu.Unlock()
		return obs.count == 3
	}, time.Second, time.Millisecond)
}

func TestWaitForJobs__blocks_until_finished(t *testing.T) {
	app := NewApp()
	go app.ProcessUpdateLoop()
	defer app.Stop()

	job := app.StartJob("slow", 0)
	done := make(chan bool)
	go func() {
		app.WaitForJobs()
		done <- true
	}()
	select {
	case <-done:
		t.Fatal("returned while a job was running")
	case <-time.After(20 * time.Millisecond):
	}
	job.Finish()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("did not return after the job finished")
	}
}

func TestFlush(t *testing.T) {
	app := NewApp()
	go app.ProcessUpdateLoop()
	defer app.Stop()

	app.AddReplaceResults(MakeResult(NS{}, "x", "x")) // not waited on
	app.Flush()
	assert.True(t, app.HasResult("x"))
}
