package ops

import (
	"sync"
	"sync/atomic"
)

// Queue manages a list of background file-operation jobs.
type Queue struct {
	jobs   []*Job
	nextID int
	mu     sync.Mutex
}

// NewQueue creates an empty Queue.
func NewQueue() *Queue {
	return &Queue{nextID: 1}
}

// Add creates a new job and appends it to the queue (not yet started).
func (q *Queue) Add(kind JobKind, src, dst string) *Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	job := &Job{
		ID:   q.nextID,
		Kind: kind,
		Src:  src,
		Dst:  dst,
		Status: StatusPending,
	}
	q.nextID++
	q.jobs = append(q.jobs, job)
	return job
}

// Jobs returns a snapshot of all jobs (caller must not mutate elements).
func (q *Queue) Jobs() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*Job, len(q.jobs))
	copy(out, q.jobs)
	return out
}

// Running returns all jobs currently in StatusRunning.
func (q *Queue) Running() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []*Job
	for _, j := range q.jobs {
		if j.Status == StatusRunning {
			out = append(out, j)
		}
	}
	return out
}

// Get returns the job with the given ID.
func (q *Queue) Get(id int) (*Job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		if j.ID == id {
			return j, true
		}
	}
	return nil, false
}

// PauseJob marks the job as paused so its goroutine will sleep between chunks.
func (q *Queue) PauseJob(id int) {
	j, ok := q.Get(id)
	if !ok {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Status == StatusRunning {
		j.paused = true
		j.Status = StatusPaused
	}
}

// ResumeJob clears the paused flag so the job goroutine continues.
func (q *Queue) ResumeJob(id int) {
	j, ok := q.Get(id)
	if !ok {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Status == StatusPaused {
		j.paused = false
		j.Status = StatusRunning
	}
}

// CancelJob signals the job's goroutine to stop.
func (q *Queue) CancelJob(id int) {
	j, ok := q.Get(id)
	if !ok {
		return
	}
	// Clear paused so the goroutine isn't sleeping when cancel is checked.
	j.mu.Lock()
	j.paused = false
	j.mu.Unlock()

	atomic.StoreUint32(&j.cancel, 1)
}

// ClearDone removes all jobs with StatusDone or StatusError from the queue.
func (q *Queue) ClearDone() {
	q.mu.Lock()
	defer q.mu.Unlock()
	filtered := q.jobs[:0]
	for _, j := range q.jobs {
		if j.Status != StatusDone && j.Status != StatusError {
			filtered = append(filtered, j)
		}
	}
	q.jobs = filtered
}
