package ops

import (
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mogglemoss/pelorus/internal/provider"
)

// JobKind identifies what kind of file operation a job performs.
type JobKind string

const (
	KindCopy   JobKind = "Copy"
	KindMove   JobKind = "Move"
	KindDelete JobKind = "Delete"
)

// JobStatus describes the lifecycle state of a job.
type JobStatus string

const (
	StatusPending JobStatus = "pending"
	StatusRunning JobStatus = "running"
	StatusDone    JobStatus = "done"
	StatusError   JobStatus = "error"
	StatusPaused  JobStatus = "paused"
)

// Job represents a single background file operation.
type Job struct {
	ID         int
	Kind       JobKind
	Src        string
	Dst        string // empty for delete
	Status     JobStatus
	Progress   float64 // 0.0–1.0
	BytesDone  int64
	BytesTotal int64
	Speed      float64 // bytes/sec, rolling average
	ETA        time.Duration
	Err        error
	StartTime  time.Time

	mu      sync.Mutex
	paused  bool
	cancel  uint32 // atomic flag: 1 = cancelled

	// rolling speed window (last 5 ticks)
	speedSamples [5]float64
	sampleIdx    int
	lastTickTime time.Time
	lastTickDone int64
}

func (j *Job) isPaused() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.paused
}

func (j *Job) isCancelled() bool {
	return atomic.LoadUint32(&j.cancel) == 1
}

func (j *Job) setCancelled() {
	atomic.StoreUint32(&j.cancel, 1)
}

func (j *Job) updateSpeed() {
	now := time.Now()
	if j.lastTickTime.IsZero() {
		j.lastTickTime = now
		j.lastTickDone = j.BytesDone
		return
	}
	elapsed := now.Sub(j.lastTickTime).Seconds()
	if elapsed <= 0 {
		return
	}
	delta := j.BytesDone - j.lastTickDone
	sample := float64(delta) / elapsed

	j.speedSamples[j.sampleIdx%5] = sample
	j.sampleIdx++

	// Average of populated samples.
	sum := 0.0
	count := 0
	for _, s := range j.speedSamples {
		if s > 0 {
			sum += s
			count++
		}
	}
	if count > 0 {
		j.Speed = sum / float64(count)
	}

	if j.BytesTotal > 0 && j.Speed > 0 {
		remaining := j.BytesTotal - j.BytesDone
		j.ETA = time.Duration(float64(remaining)/j.Speed) * time.Second
	}

	if j.BytesTotal > 0 {
		j.Progress = float64(j.BytesDone) / float64(j.BytesTotal)
	}

	j.lastTickTime = now
	j.lastTickDone = j.BytesDone
}

// ProgressMsg is sent periodically by a running job.
type ProgressMsg struct {
	JobID int
	Done  int64
	Total int64
	Speed float64
	ETA   time.Duration
}

// JobDoneMsg is sent when a job completes (success or error).
type JobDoneMsg struct {
	JobID int
	Err   error
}

// DeleteConfirmedMsg is sent by the pane when the user confirms deletion.
type DeleteConfirmedMsg struct {
	Path     string
	Provider provider.Provider
}

// StartJob returns a tea.Cmd that runs the job in a goroutine and sends a
// final JobDoneMsg when done. Progress is polled separately via TickProgress.
func StartJob(job *Job, srcProv, dstProv provider.Provider) tea.Cmd {
	return func() tea.Msg {
		err := runJob(job, srcProv, dstProv)
		return JobDoneMsg{JobID: job.ID, Err: err}
	}
}

// TickProgress returns a Cmd that snapshots progress for the given job and
// sends a ProgressMsg. The app calls this on a ticker.
func TickProgress(job *Job) tea.Cmd {
	return func() tea.Msg {
		return ProgressMsg{
			JobID: job.ID,
			Done:  job.BytesDone,
			Total: job.BytesTotal,
			Speed: job.Speed,
			ETA:   job.ETA,
		}
	}
}

// runJob executes the job synchronously (called from inside a tea.Cmd goroutine).
func runJob(job *Job, srcProv, dstProv provider.Provider) error {
	switch job.Kind {
	case KindCopy:
		return doCopy(job, srcProv)
	case KindMove:
		return doMove(job, srcProv, dstProv)
	case KindDelete:
		return doDelete(job, srcProv)
	}
	return errors.New("unknown job kind")
}

func doCopy(job *Job, srcProv provider.Provider) error {
	rc, err := srcProv.Read(job.Src)
	if err != nil {
		return err
	}
	defer rc.Close()

	// Determine total size for progress.
	if fi, err2 := srcProv.Stat(job.Src); err2 == nil {
		job.BytesTotal = fi.Size
	}

	// Ensure destination directory exists.
	dst := job.Dst
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	return copyWithProgress(job, rc, f, job.BytesTotal)
}

func doMove(job *Job, srcProv, dstProv provider.Provider) error {
	// Fast path: try provider's Move.
	if err := srcProv.Move(job.Src, job.Dst); err == nil {
		job.BytesDone = 1
		job.BytesTotal = 1
		job.Progress = 1.0
		return nil
	}
	// Slow path: copy then delete.
	if err := doCopy(job, srcProv); err != nil {
		return err
	}
	return srcProv.Delete(job.Src)
}

func doDelete(job *Job, srcProv provider.Provider) error {
	err := srcProv.Delete(job.Src)
	job.BytesDone = 1
	job.BytesTotal = 1
	job.Progress = 1.0
	return err
}

func copyWithProgress(job *Job, src io.Reader, dst io.Writer, total int64) error {
	buf := make([]byte, 32*1024)
	var lastReport time.Time

	for {
		if job.isCancelled() {
			return errors.New("cancelled")
		}
		for job.isPaused() {
			time.Sleep(50 * time.Millisecond)
			if job.isCancelled() {
				return errors.New("cancelled")
			}
		}

		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
			job.BytesDone += int64(n)
			if time.Since(lastReport) > 100*time.Millisecond {
				job.updateSpeed()
				lastReport = time.Now()
			}
		}
		if err == io.EOF {
			// Final progress update.
			job.updateSpeed()
			return nil
		}
		if err != nil {
			return err
		}
	}
}
