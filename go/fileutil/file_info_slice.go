package fileutil

import (
	"os"
	"sort"

	"github.com/skia-dev/glog"
)

// FileInfoSlice is a sortable slice of os.FileInfo.  It has some convenience methods on it, like ContainsFileByName()
type FileInfoSlice []os.FileInfo

func (s FileInfoSlice) Len() int           { return len(s) }
func (s FileInfoSlice) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
func (s FileInfoSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s FileInfoSlice) Sort() {
	if !sort.IsSorted(s) {
		sort.Sort(s)
	}
}

// ContainsFileByName returns true if this FileInfoSlice has a file with the given name.  It uses binary search, and must be sorted (via .Sort()) prior to invocation.
func (s FileInfoSlice) ContainsFileByName(name string) bool {
	i := sort.Search(s.Len(), func(i int) bool {
		return s[i].Name() >= name
	})
	if i < s.Len() && s[i].Name() == name {
		return true
	}
	return false
}

// LogFileInfo logs the FileInfoSLice in human readable form, namely file name and if it is a directory or not
func (s FileInfoSlice) LogFileInfo() {
	glog.Infof("Slice contains %d file elements", len(s))
	for _, fi := range s {
		glog.Infof("Name %s, Is directory: %t", fi.Name(), fi.IsDir())
	}
	glog.Info("End File Infos")
}
