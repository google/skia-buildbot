package modified

import (
	"time"

	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

// NewMuxModifiedData returns a db.ModifiedData implementation which writes to
// multiple ModifiedData instances but only reads from one.
func NewMuxModifiedData(readWrite db.ModifiedData, writeOnly ...db.ModifiedData) db.ModifiedData {
	tWriteOnly := make([]db.ModifiedTasks, 0, len(writeOnly))
	jWriteOnly := make([]db.ModifiedJobs, 0, len(writeOnly))
	cWriteOnly := make([]db.ModifiedComments, 0, len(writeOnly))
	for _, wo := range writeOnly {
		tWriteOnly = append(tWriteOnly, wo)
		jWriteOnly = append(jWriteOnly, wo)
		cWriteOnly = append(cWriteOnly, wo)
	}
	t := NewMuxModifiedTasks(readWrite, tWriteOnly...)
	j := NewMuxModifiedJobs(readWrite, jWriteOnly...)
	c := NewMuxModifiedComments(readWrite, cWriteOnly...)
	return db.NewModifiedData(t, j, c)
}

// MuxModifiedTasks is an implementation of db.ModifiedTasks which writes to
// multiple ModifiedTasks instances but only reads from one.
type MuxModifiedTasks struct {
	db.ModifiedTasks
	writeOnly []db.ModifiedTasks
}

// NewMuxModifiedTasks returns an implementation of db.ModifiedTasks which
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

// MuxModifiedComments is an implementation of db.ModifiedComments which writes
// to multiple ModifiedJobs instances but only reads from one.
type MuxModifiedComments struct {
	db.ModifiedComments
	writeOnly []db.ModifiedComments
}

// New MuxModifiedComments returns an implementation of db.ModifiedComments
// which writes to multiple ModifiedJobs instances but only reads from one.
func NewMuxModifiedComments(readWrite db.ModifiedComments, writeOnly ...db.ModifiedComments) db.ModifiedComments {
	return &MuxModifiedComments{
		ModifiedComments: readWrite,
		writeOnly:        writeOnly,
	}
}

// See documentation for db.ModifiedComments interface.
func (m *MuxModifiedComments) TrackModifiedTaskComment(c *types.TaskComment) {
	m.ModifiedComments.TrackModifiedTaskComment(c)
	for _, wo := range m.writeOnly {
		wo.TrackModifiedTaskComment(c)
	}
}

// See documentation for db.ModifiedComments interface.
func (m *MuxModifiedComments) TrackModifiedTaskSpecComment(c *types.TaskSpecComment) {
	m.ModifiedComments.TrackModifiedTaskSpecComment(c)
	for _, wo := range m.writeOnly {
		wo.TrackModifiedTaskSpecComment(c)
	}
}

// See documentation for db.ModifiedComments interface.
func (m *MuxModifiedComments) TrackModifiedCommitComment(c *types.CommitComment) {
	m.ModifiedComments.TrackModifiedCommitComment(c)
	for _, wo := range m.writeOnly {
		wo.TrackModifiedCommitComment(c)
	}
}
