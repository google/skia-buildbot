// Application that serves up the contents of /tmp/glog via HTTP, giving access
// to logs w/o needing to SSH into the server.
package main

import (
	"bufio"
	"encoding/gob"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/go/common"
	"skia.googlesource.com/buildbot.git/go/util"
)

var (
	port           = flag.String("port", ":10115", "HTTP service address (e.g., ':10115')")
	dir            = flag.String("dir", "/tmp/glog", "Directory to serve log files from.")
	graphiteServer = flag.String("graphite_server", "skiamonitor.com:2003", "Where is Graphite metrics ingestion server running.")
	stateFile      = flag.String("state_file", "/tmp/logserver.state", "File where logserver stores all encountered log files. This ensures that metrics are not duplicated for already processed log files.")

	appLogThreshold = flag.Int64(
		"app_log_threshold", 1<<30,
		"If any app's logs for a log level use up more than app_log_threshold value then the files with the oldest modified time are deleted till size is less than app_log_threshold - app_log_threshold_buffer.")
	appLogThresholdBuffer = flag.Int64(
		"app_log_threshold_buffer", 10*1<<20,
		"If any app's logs for a log level use up more than app_log_threshold then the files with the oldest modified time are deleted till size is less than app_log_threshold - app_log_threshold_buffer.")
	dirWatchDuration = flag.Duration("dir_watch_duration", 10*time.Second, "How long dir watcher sleeps for before checking the dir.")
)

// FileServer returns a handler that serves HTTP requests
// with the contents of the file system rooted at root.
//
// To use the operating system's file system implementation,
// use http.Dir:
//
//     http.Handle("/", FileServer(http.Dir("/tmp")))
//
// Differs from net/http FileServer by making directory listings better.
func FileServer(root http.FileSystem) http.Handler {
	return &fileHandler{root}
}

type fileHandler struct {
	root http.FileSystem
}

func (f *fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	serveFile(w, r, f.root, path.Clean(upath))
}

// FileInfoNameSlice is for sorting files by their names.
type FileInfoNameSlice []os.FileInfo

func (p FileInfoNameSlice) Len() int           { return len(p) }
func (p FileInfoNameSlice) Less(i, j int) bool { return p[i].Name() < p[j].Name() }
func (p FileInfoNameSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// FileInfoModifiedSlice is for sorting files by their modified time.
type FileInfoModifiedSlice []os.FileInfo

func (p FileInfoModifiedSlice) Len() int           { return len(p) }
func (p FileInfoModifiedSlice) Less(i, j int) bool { return p[i].ModTime().Before(p[j].ModTime()) }
func (p FileInfoModifiedSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// dirList writes the directory list to the HTTP response.
//
// glog convention is that log files are created in the following format:
// "ingest.skia-testing-b.perf.log.ERROR.20141015-133007.3273"
// where the first word is the name of the app.
// glog also creates symlinks that look like "ingest.ERROR". These
// symlinks point to the latest log type.
// This method displays sorted symlinks first and then displays sorted sections for
// all apps. Files and directories not in the glog format are bucketed into an
// "unknown" app.
func dirList(w http.ResponseWriter, f http.File) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<pre>\n")
	// Datastructures to populate and output.
	topLevelSymlinks := make([]os.FileInfo, 0)
	appToLogs := make(map[string][]os.FileInfo)
	for {
		fileInfos, err := f.Readdir(10000)
		if err != nil || len(fileInfos) == 0 {
			break
		}
		// Prepopulate the datastructures.
		for _, fileInfo := range fileInfos {
			name := fileInfo.Name()
			nameTokens := strings.Split(name, ".")
			if len(nameTokens) == 2 {
				topLevelSymlinks = append(topLevelSymlinks, fileInfo)
			} else if len(nameTokens) > 1 {
				appToLogs[nameTokens[0]] = append(appToLogs[nameTokens[0]], fileInfo)
			} else {
				// File all directories or files created by something other than
				// glog under "unknown" app.
				appToLogs["unknown"] = append(appToLogs["unknown"], fileInfo)
			}
		}
	}
	// First output the top level symlinks.
	sort.Sort(FileInfoNameSlice(topLevelSymlinks))
	for _, fileInfo := range topLevelSymlinks {
		writeFileInfo(w, fileInfo)
	}
	// Second output app links to their anchors.
	var keys []string
	for k := range appToLogs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) != 0 {
		fmt.Fprint(w, "\nJump to sections:\n")
	}
	for _, app := range keys {
		fmt.Fprintf(w, "<a href=\"#%s\">%s</a>\n", app, template.HTMLEscapeString(app))
	}
	fmt.Fprint(w, "\n")
	// Then output the logs of all the different apps.
	for _, app := range keys {
		appFileInfos := appToLogs[app]
		sort.Sort(FileInfoNameSlice(appFileInfos))
		fmt.Fprintf(w, "\n===== <a name=\"%s\">%s</a> =====\n\n", app, template.HTMLEscapeString(app))
		for _, fileInfo := range appFileInfos {
			writeFileInfo(w, fileInfo)
		}
	}
	fmt.Fprintf(w, "</pre>\n")
}

func writeFileInfo(w http.ResponseWriter, fileInfo os.FileInfo) {
	name := fileInfo.Name()
	if fileInfo.IsDir() {
		name += "/"
	}

	url := url.URL{Path: name}
	downloadLink := ""
	if !fileInfo.IsDir() {
		fileSize := util.GetFormattedByteSize(float64(fileInfo.Size()))
		downloadLink = fmt.Sprintf("(%s <a href=\"%s\" download=\"%s\">download</a>)", fileSize, url.String(), template.HTMLEscapeString(name))
	}
	fmt.Fprintf(w, "%s <a href=\"%s\">%s</a>    %s\n", fileInfo.ModTime(), url.String(), template.HTMLEscapeString(name), downloadLink)
}

func serveFile(w http.ResponseWriter, r *http.Request, fs http.FileSystem, name string) {
	f, err := fs.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	d, err1 := f.Stat()
	if err1 != nil {
		http.NotFound(w, r)
		return
	}

	url := r.URL.Path
	if d.IsDir() {
		if url[len(url)-1] != '/' {
			w.Header().Set("Location", path.Base(url)+"/")
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}
	}

	if d.IsDir() {
		glog.Infof("Dir List: %s", name)
		dirList(w, f)
		return
	}

	http.ServeContent(w, r, d.Name(), d.ModTime(), f)
}

// getAppAndLogLevel returns the app name and the log level of the specified
// glog file by parsing it.
// It expects a structure that looks like this:
// "ingest.skia-testing-b.perf.log.ERROR.20141015-133007.3273"
func getAppAndLogLevel(fileInfo os.FileInfo) (string, string) {
	name := fileInfo.Name()
	nameTokens := strings.Split(name, ".")
	if len(nameTokens) > 5 {
		return nameTokens[0], nameTokens[4]
	}
	// Ignore symlinks and other logs not created by glog.
	return "", ""
}

type fileState struct {
	LineCount int64
	Size      int64
}

type logserverState struct {
	FilesToState       map[string]fileState
	AppLogLevelToSpace map[string]int64
	AppLogLevelToCount map[string]int64
	LastCompletedRun   time.Time
}

func getPreviousState() (map[string]fileState, map[string]int64, map[string]int64, time.Time, error) {
	if _, err := os.Stat(*stateFile); os.IsNotExist(err) {
		// State file does not exist, return empty values.
		return map[string]fileState{}, map[string]int64{}, map[string]int64{}, time.Time{}, nil
	}
	f, err := os.Open(*stateFile)
	if err != nil {
		return nil, nil, nil, time.Time{}, fmt.Errorf("Failed to open state file %s for reading: %s", *stateFile, err)
	}
	defer f.Close()
	state := &logserverState{}
	dec := gob.NewDecoder(f)
	if err := dec.Decode(state); err != nil {
		return nil, nil, nil, time.Time{}, fmt.Errorf("Failed to decode state file: %s", err)
	}
	return state.FilesToState, state.AppLogLevelToSpace, state.AppLogLevelToCount, state.LastCompletedRun, nil
}

func writeCurrentState(filestoState map[string]fileState, appLogLevelToSpace, appLogLevelToCount map[string]int64, lastCompletedRun time.Time) error {
	f, err := os.Create(*stateFile)
	if err != nil {
		return fmt.Errorf("Unable to create state file %s: %s", *stateFile, err)
	}
	defer f.Close()
	state := &logserverState{
		FilesToState:       filestoState,
		AppLogLevelToSpace: appLogLevelToSpace,
		AppLogLevelToCount: appLogLevelToCount,
		LastCompletedRun:   lastCompletedRun,
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(state); err != nil {
		return fmt.Errorf("Failed to encode state: %s", err)
	}
	return nil
}

func getLineCount(path string) int64 {
	file, _ := os.Open(path)
	fileScanner := bufio.NewScanner(file)
	var lineCount int64
	for fileScanner.Scan() {
		lineCount++
	}
	return lineCount
}

// dirWatcher watches for changes in the specified dir. The frequency of polling
// is determined by the duration parameter. dirWatcher ensures:
// * Each app's logs do not exceed the log limit threshold. If they do then the
//   oldest files are deleted.
// * New encountered logs are reported to InfluxDB.
func dirWatcher(duration time.Duration, dir string) {
	filesToState, appLogLevelToSpace, appLogLevelToCount, lastCompletedRun, err := getPreviousState()
	if err != nil {
		glog.Fatalf("Could get access previous state: %s", err)
	}
	appLogLevelToMetric := make(map[string]metrics.Gauge)
	updatedFiles := false
	markFn := func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fileInfo.IsDir() || fileInfo.Mode()&os.ModeSymlink != 0 {
			// We are only interested in watching non-symlink log files in the
			// top-level dir.
			return nil
		}

		if _, exists := filesToState[path]; !exists || fileInfo.ModTime().After(lastCompletedRun) {
			glog.Infof("Processing %s", path)
			app, logLevel := getAppAndLogLevel(fileInfo)
			if app != "" && logLevel != "" {
				appLogLevel := fmt.Sprintf("%s.%s", app, logLevel)
				if _, ok := appLogLevelToMetric[appLogLevel]; !ok {
					// First time encountered this app and log level combination.
					// Create a counter metric.
					appLogLevelToMetric[appLogLevel] = metrics.NewRegisteredGauge("logserver."+appLogLevel, metrics.DefaultRegistry)
				}

				// Calculate how many new lines and new disk space usage there is.
				totalLines := getLineCount(path)
				totalSize := fileInfo.Size()
				newLines := totalLines
				newSpace := totalSize
				if exists {
					fileState := filesToState[path]
					newLines = totalLines - fileState.LineCount
					newSpace = totalSize - fileState.Size
				}

				glog.Infof("Processed %d new lines", newLines)
				glog.Infof("Processed %d new bytes", newSpace)

				// Update the logs count metric.
				appLogLevelToCount[appLogLevel] += newLines
				appLogLevelToMetric[appLogLevel].Update(appLogLevelToCount[appLogLevel])

				// Add the file size to the current space count for this app and
				// log level combination.
				appLogLevelToSpace[appLogLevel] += newSpace

				updatedFiles = true
			}
			filesToState[path] = fileState{LineCount: getLineCount(path), Size: fileInfo.Size()}
		}
		return nil
	}

	for _ = range time.Tick(duration) {
		filepath.Walk(dir, markFn)
		deletedFiles := cleanupAppLogs(dir, appLogLevelToSpace, filesToState)
		if updatedFiles || deletedFiles {
			if err := writeCurrentState(filesToState, appLogLevelToSpace, appLogLevelToCount, time.Now()); err != nil {
				glog.Fatalf("Could not write state: %s", err)
			}
			glog.Info(getPrettyMap(appLogLevelToCount, "AppLogLevels to their line counts"))
			glog.Info(getPrettyMap(appLogLevelToSpace, "AppLogLevels to their disk space"))
		}
		updatedFiles = false
		lastCompletedRun = time.Now()
	}
}

func getPrettyMap(m map[string]int64, name string) string {
	log := name + ": {"
	for k := range m {
		log += fmt.Sprintf("%s: %d, ", k, m[k])
	}
	log = strings.TrimRight(log, ", ")
	log += "}"
	return log
}

func cleanupAppLogs(dir string, appLogLevelToSpace map[string]int64, filesToState map[string]fileState) bool {
	deletedFiles := false
	for appLogLevel := range appLogLevelToSpace {
		if appLogLevelToSpace[appLogLevel] > *appLogThreshold {
			glog.Infof("App %s is above the threshold. Usage: %d. Threshold: %d", appLogLevel, appLogLevelToSpace[appLogLevel], *appLogThreshold)
			tokens := strings.Split(appLogLevel, ".")
			app := tokens[0]
			logLevel := tokens[1]
			logGlob := filepath.Join(dir, app+".*"+logLevel+".*")
			matches, err := filepath.Glob(logGlob)
			if err != nil {
				glog.Fatalf("Could not glob for %s: %s", logGlob, err)
			}
			fileInfos := make([]os.FileInfo, len(matches))
			for i, match := range matches {
				fileInfo, err := os.Stat(match)
				if err != nil {
					glog.Fatalf("Could not stat %s: %s", match, err)
				}
				fileInfos[i] = fileInfo
			}
			// Sort by Modified time and keep deleting till we are at
			// (threshold - buffer) space left.
			sort.Sort(FileInfoModifiedSlice(fileInfos))
			index := 0
			for appLogLevelToSpace[appLogLevel] > *appLogThreshold-*appLogThresholdBuffer {
				fileName := fileInfos[index].Name()
				appLogLevelToSpace[appLogLevel] -= fileInfos[index].Size()
				if err = os.Remove(filepath.Join(dir, fileName)); err != nil {
					glog.Fatalf("Could not delete %s: %s", fileName, err)
				}
				// Remove the entry from the filesToState map.
				delete(filesToState, filepath.Join(dir, fileName))
				deletedFiles = true
				glog.Infof("Deleted %s", fileName)
				index++
			}
			// Just incase we delete a massive log file.
			if appLogLevelToSpace[appLogLevel] < 0 {
				appLogLevelToSpace[appLogLevel] = 0
			}
		}
	}
	return deletedFiles
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		glog.Fatalf("Failed to get Hostname: %s", err)
	}
	appName := "logserver." + hostname
	common.InitWithMetrics(appName, graphiteServer)

	if err := os.MkdirAll(*dir, 0777); err != nil {
		glog.Fatalf("Failed to create dir for log files: %s", err)
	}

	go dirWatcher(*dirWatchDuration, *dir)

	http.Handle("/", http.StripPrefix("/", FileServer(http.Dir(*dir))))
	glog.Fatal(http.ListenAndServe(*port, nil))
}
