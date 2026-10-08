package core

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// background jobs: named units of long-running work whose progress the app shows.
// jobs are kept apart from the result list because they change often and are small:
// routing every progress step through `UpdateState` would clone the whole state each time.
// every change is announced to observers with an `ACTION_JOBS_CHANGED` action, through
// the same channel as state updates, so it is ordered with them.

// announced to observers whenever a job starts, progresses or finishes.
// the payload is nil: observers read the current jobs with `App.Jobs`.
var ACTION_JOBS_CHANGED ActionType = "jobs-changed"

// a snapshot of one running job.
type JobInfo struct {
	ID      string
	Name    string    // "checking for updates"
	Done    int       // steps finished
	Total   int       // steps expected, zero when not known
	Started time.Time // for a stable display order
}

// the running jobs, by job ID.
type job_store struct {
	mu    sync.Mutex
	jobs  map[string]JobInfo // a map: jobs are looked up and removed by ID
	count atomic.Int64       // for unique job IDs
	idle  *sync.Cond         // broadcast when the last job finishes
}

func new_job_store() *job_store {
	js := &job_store{jobs: map[string]JobInfo{}}
	js.idle = sync.NewCond(&js.mu)
	return js
}

// a handle on a running job, returned by `App.StartJob`.
// every method is safe to call from any goroutine and after the job has finished.
type Job struct {
	app      *App
	id       string
	finished atomic.Bool
}

// starts a job named `name` expecting `total` steps (zero when not known) and returns
// its handle. callers should `defer job.Finish()`.
func (app *App) StartJob(name string, total int) *Job {
	js := app.jobs
	id := name + "#" + IntToString(int(js.count.Add(1)))
	js.mu.Lock()
	js.jobs[id] = JobInfo{ID: id, Name: name, Total: total, Started: time.Now()}
	js.mu.Unlock()
	app.announce_jobs()
	return &Job{app: app, id: id}
}

// changes a job's progress with `fn`, unless it has finished.
func (j *Job) update(fn func(JobInfo) JobInfo) {
	if j.finished.Load() {
		return
	}
	js := j.app.jobs
	js.mu.Lock()
	info, present := js.jobs[j.id]
	if present {
		js.jobs[j.id] = fn(info)
	}
	js.mu.Unlock()
	if present {
		j.app.announce_jobs()
	}
}

// records `n` more steps done.
func (j *Job) Tick(n int) {
	j.update(func(info JobInfo) JobInfo {
		info.Done += n
		return info
	})
}

// sets the number of steps expected, for jobs that only learn it once started.
func (j *Job) SetTotal(total int) {
	j.update(func(info JobInfo) JobInfo {
		info.Total = total
		return info
	})
}

// ends the job. calling it again does nothing.
func (j *Job) Finish() {
	if j.finished.Swap(true) {
		return
	}
	js := j.app.jobs
	js.mu.Lock()
	delete(js.jobs, j.id)
	if len(js.jobs) == 0 {
		js.idle.Broadcast()
	}
	js.mu.Unlock()
	j.app.announce_jobs()
}

// returns the running jobs, oldest first.
func (app *App) Jobs() []JobInfo {
	js := app.jobs
	js.mu.Lock()
	job_list := []JobInfo{}
	for _, info := range js.jobs {
		job_list = append(job_list, info)
	}
	js.mu.Unlock()
	slices.SortFunc(job_list, func(a, b JobInfo) int {
		if c := a.Started.Compare(b.Started); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return job_list
}

// blocks until no jobs are running.
func (app *App) WaitForJobs() {
	js := app.jobs
	js.mu.Lock()
	for len(js.jobs) > 0 {
		js.idle.Wait()
	}
	js.mu.Unlock()
}

// tells observers the jobs changed, without waiting for them to be told.
// the announcement is queued from a new goroutine, so a job can be started or ticked from
// anywhere, including from inside a state update, without deadlocking.
func (app *App) announce_jobs() {
	if app.stopped.Load() {
		return
	}
	action := Action{Type: ACTION_JOBS_CHANGED}
	go func() {
		var wg sync.WaitGroup
		wg.Add(1)
		app.enqueue(StateUpdate{Action: &action, Wg: &wg})
	}()
}

// announced by `Flush`. observers can ignore it.
var ACTION_FLUSH ActionType = "flush"

// blocks until every state update queued before this call has been applied and its
// observers notified.
// updates are processed one at a time, in order, so once this action is processed every
// earlier update has finished.
func (app *App) Flush() {
	app.DispatchAction(Action{Type: ACTION_FLUSH})
}
