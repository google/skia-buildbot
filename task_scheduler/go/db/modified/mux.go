package modified

import (
	"time"

	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

// MuxModifiedTasks is an implementation of db.ModifiedTasks which writes to
// multiple ModifiedTasks instances but only reads from one.
type MuxModifiedTasks struct {
	db.ModifiedTasks
	writeOnly []db.ModifiedTasks
}

// New MuxModifiedTasks returns an implementation of db.ModifiedTasks which
// writes to multiple ModifiedTasks instances but only reads from one.
func NewMuxModifiedTasks(readWrite db.ModifiedTasks, writeOnly ...db.ModifiedTasks) db.ModifiedTasks {
	return &MuxModifiedTasks{
		ModifiedTasks: readWrite,
		writeOnly:     writeOnly,
	}
}

// See documentation for db.ModifiedTasks interface.
func (m *MuxModifiedTasks) TrackModifiedTask(task *types.Task) {
	m.ModifiedTasks.TrackModifiedTask(task)
	for _, wo := range m.writeOnly {
		wo.TrackModifiedTask(task)
	}
}

// See documentation for db.ModifiedTasks interface.
func (m *MuxModifiedTasks) TrackModifiedTasksGOB(dbModified time.Time, gobs map[string][]byte) {
	m.ModifiedTasks.TrackModifiedTasksGOB(dbModified, gobs)
	for _, wo := range m.writeOnly {
		wo.TrackModifiedTasksGOB(dbModified, gobs)
	}
}

// MuxModifiedJobs is an implementation of db.ModifiedJobs which writes to
// multiple ModifiedJobs instances but only reads from one.
type MuxModifiedJobs struct {
	db.ModifiedJobs
	writeOnly []db.ModifiedJobs
}

// New MuxModifiedJobs returns an implementation of db.ModifiedJobs which
// writes to multiple ModifiedJobs instances but only reads from one.
func NewMuxModifiedJobs(readWrite db.ModifiedJobs, writeOnly ...db.ModifiedJobs) db.ModifiedJobs {
	return &MuxModifiedJobs{
		ModifiedJobs: readWrite,
		writeOnly:    writeOnly,
	}
}

// See documentation for db.ModifiedJobs interface.
func (m *MuxModifiedJobs) TrackModifiedJob(task *types.Job) {
	m.ModifiedJobs.TrackModifiedJob(task)
	for _, wo := range m.writeOnly {
		wo.TrackModifiedJob(task)
	}
}

// See documentation for db.ModifiedJobs interface.
func (m *MuxModifiedJobs) TrackModifiedJobsGOB(dbModified time.Time, gobs map[string][]byte) {
	m.ModifiedJobs.TrackModifiedJobsGOB(dbModified, gobs)
	for _, wo := range m.writeOnly {
		wo.TrackModifiedJobsGOB(dbModified, gobs)
	}
}
