package util

import (
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/zeebo/bencode"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/sync/errgroup"
)

const (
	// time.RFC3339Nano only uses as many sub-second digits are required to
	// represent the time, which makes it unsuitable for sorting. This
	// format ensures that all 9 nanosecond digits are used, padding with
	// zeroes if necessary.
	RFC3339NanoZeroPad = "2006-01-02T15:04:05.000000000Z07:00"

	// SAFE_TIMESTAMP_FORMAT is time format which is similar to
	// RFC3339NanoZeroPad, but with most of the punctuation omitted. This
	// timestamp can only be used to format and parse times in UTC.
	SAFE_TIMESTAMP_FORMAT = "20060102T150405.000000000Z"
)

var (
	timeUnixZero = time.Unix(0, 0).UTC()
)

// In returns true if |s| is *in* |a| slice.
func In(s string, a []string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

// ContainsAny returns true if |s| contains any element of |a|.
func ContainsAny(s string, a []string) bool {
	for _, x := range a {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// Index returns the index of |s| *in* |a| slice, and -1 if not found.
func Index(s string, a []string) int {
	for i, x := range a {
		if x == s {
			return i
		}
	}
	return -1
}

// SSliceEqual returns true if the given string slices are equal
func SSliceEqual(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i, aa := range a {
		if aa != b[i] {
			return false
		}
	}
	return true
}

// Reverse returns the given slice of strings in reverse order.
func Reverse(s []string) []string {
	r := make([]string, 0, len(s))
	for i := len(s) - 1; i >= 0; i-- {
		r = append(r, s[i])
	}
	return r
}

// insertString inserts the given string into the slice at the given index.
func insertString(strs []string, idx int, s string) []string {
	oldLen := len(strs)
	strs = append(strs, "")
	copy(strs[idx+1:], strs[idx:oldLen])
	strs[idx] = s
	return strs
}

// InsertStringSorted inserts the given string into the sorted slice of strings
// if it does not already exist. Maintains sorted order.
func InsertStringSorted(strs []string, s string) []string {
	idx := sort.SearchStrings(strs, s)
	if idx == len(strs) || strs[idx] != s {
		return insertString(strs, idx, s)
	}
	return strs
}

type Int64Slice []int64

func (p Int64Slice) Len() int           { return len(p) }
func (p Int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// MaxInt returns the largest integer of the arguments provided.
func MaxInt(intList ...int) int {
	ret := intList[0]
	for _, i := range intList[1:] {
		if i > ret {
			ret = i
		}
	}
	return ret
}

// MaxInt64 returns largest integer of a and b.
func MaxInt64(a, b int64) int64 {
	if a < b {
		return b
	}
	return a
}

// MaxInt32 returns largest integer of a and b.
func MaxInt32(a, b int32) int32 {
	if a < b {
		return b
	}
	return a
}

// MinInt returns the smaller integer of a and b.
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MinInt64 returns the smaller integer of a and b.
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// MinInt32 returns the smaller integer of a and b.
func MinInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// AbsInt returns the absolute value of v.
func AbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TimeStampMs returns the current time in milliseconds since the epoch.
func TimeStampMs() int64 {
	return TimeStamp(time.Millisecond)
}

// TimeStamp returns the current time in the units defined by the given target unit.
// e.g. TimeStamp(time.Millisecond) will return the time in Milliseconds.
// The result is always rounded down to the lowest integer from the
// representation in nano seconds.
func TimeStamp(targetUnit time.Duration) int64 {
	return time.Now().UnixNano() / int64(targetUnit)
}

// RepeatJoin repeats a given string N times with the given separator between
// each instance.
func RepeatJoin(str, sep string, n int) string {
	if n <= 0 {
		return ""
	}
	return str + strings.Repeat(sep+str, n-1)
}

// AddParams adds the second instance of map[string]string to the first and
// returns the first map.
func AddParams(a map[string]string, b ...map[string]string) map[string]string {
	if a == nil {
		a = make(map[string]string, len(b))
	}
	for _, oneMap := range b {
		for k, v := range oneMap {
			a[k] = v
		}
	}
	return a
}

// CopyStringMap returns a copy of the provided map[string]string such that
// reflect.DeepEqual returns true for the given map and the returned map. In
// particular, preserves nil input.
func CopyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	ret := make(map[string]string, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

// CopyStringSlice copies the given []string such that reflect.DeepEqual returns
// true for the given slice and the returned slice. In particular, preserves
// nil slice input.
func CopyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	rv := make([]string, len(s))
	copy(rv, s)
	return rv
}

// CopyString returns a copy of the given string. This may seem unnecessary, but
// is very important at preventing leaks of strings. For example, subslicing
// a string can prevent the larger string from being cleaned up.
func CopyString(s string) string {
	if len(s) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString(s)
	return b.String()
}

// CleanupFunc is a function return value that can be deferred by the caller to
// clean up any resources created/acquired by the function.
type CleanupFunc func()

// Close wraps an io.Closer and logs an error if one is returned. When
// manipulating the file, prefer util.WithReadFile over util.Close, as
// it handles closing automatically.
func Close(c io.Closer) {
	if err := c.Close(); err != nil {
		// Don't start the stacktrace here, but at the caller's location
		sklog.ErrorfWithDepth(1, "Failed to Close(): %v", err)
	}
}

// RemoveAll removes the specified path and logs an error if one is returned.
func RemoveAll(path string) {
	if err := os.RemoveAll(path); err != nil {
		sklog.ErrorfWithDepth(1, "Failed to RemoveAll(%s): %v", path, err)
	}
}

// Remove removes the specified file and logs an error if one is returned.
func Remove(name string) {
	if err := os.Remove(name); err != nil {
		sklog.ErrorfWithDepth(1, "Failed to Remove(%s): %v", name, err)
	}
}

// LogErr logs err if it's not nil. This is intended to be used
// for calls where generally a returned error can be ignored.
func LogErr(err error) {
	if err != nil {
		sklog.ErrorfWithDepth(1, "Unexpected error: %s", err)
	}
}

// IsNil returns true if i is nil or is an interface containing a nil or invalid value.
func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return true
		}
		inner := v.Elem()
		if !inner.IsValid() {
			return true
		}
		if inner.CanInterface() {
			return IsNil(inner.Interface())
		}
		return false
	default:
		return false
	}
}

// MD5Sum returns the MD5 hash of the given value. It supports anything that
// can be encoded via bencode (https://en.wikipedia.org/wiki/Bencode).
func MD5Sum(val interface{}) (string, error) {
	md5Writer := md5.New()
	enc := bencode.NewEncoder(md5Writer)
	if err := enc.Encode(val); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5Writer.Sum(nil)), nil
}

// MD5SSlice returns the MD5 hash of the provided []string.
func MD5SSlice(val []string) (string, error) {
	md5Writer := md5.New()
	enc := bencode.NewEncoder(md5Writer)
	if err := enc.Encode(val); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5Writer.Sum(nil)), nil
}

// TimeIsZero returns true if the time.Time is a zero-value or corresponds to
// a zero Unix timestamp.
func TimeIsZero(t time.Time) bool {
	return t.IsZero() || t.UTC() == timeUnixZero
}

// Repeat calls the provided function 'fn' immediately and then in intervals
// defined by 'interval'. If anything is sent on the provided stop channel,
// the iteration stops.
func Repeat(interval time.Duration, stopCh <-chan bool, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	fn()
MainLoop:
	for {
		select {
		case <-stopCh:
			break MainLoop
		case <-ticker.C:
			fn()
		}
	}
}

// RepeatCtx calls the provided function 'fn' immediately and then in intervals
// defined by 'interval'. If the given context is canceled, the iteration stops.
func RepeatCtx(ctx context.Context, interval time.Duration, fn func(ctx context.Context)) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	done := ctx.Done()
	defer ticker.Stop()
	fn(ctx)
MainLoop:
	for {
		select {
		case <-done:
			break MainLoop
		case <-ticker.C:
			fn(ctx)
		}
	}
}

// ChunkIter iterates over a slice in chunks of smaller slices.
func ChunkIter(length, chunkSize int, fn func(startIdx int, endIdx int) error) error {
	if chunkSize < 1 {
		return skerr.Fmt("chunk size may not be less than 1 (saw %d)", chunkSize)
	}
	if length < 0 {
		return skerr.Fmt("length cannot be negative (saw %d)", length)
	}
	chunkStart := 0
	chunkEnd := MinInt(length, chunkSize)
	for {
		if err := fn(chunkStart, chunkEnd); err != nil {
			return err
		}
		if chunkEnd == length {
			return nil
		}
		chunkStart = chunkEnd
		chunkEnd = MinInt(length, chunkEnd+chunkSize)
	}
}

// ChunkIterParallel is similar to ChunkIter but it uses an errgroup to run the chunks in parallel.
// To avoid costly execution from happening after the error context is cancelled, it is recommended
// to include a context short-circuit inside of the loop processing the subslice:
// var xs []string
//
//	util.ChunkIterParallel(ctx, len(xs), 10, func(ctx context.Context, start, stop int) error {
//	  for _, tr := range xs[start:stop] {
//	    if err := ctx.Err(); err != nil {
//	      return err
//	    }
//	    // Do work here.
//	  }
//	}
func ChunkIterParallel(ctx context.Context, length, chunkSize int, fn func(ctx context.Context, startIdx int, endIdx int) error) error {
	if chunkSize < 1 {
		return skerr.Fmt("chunk size may not be less than 1 (saw %d)", chunkSize)
	}
	if length < 0 {
		return skerr.Fmt("length cannot be negative (saw %d)", length)
	}
	chunkStart := 0
	chunkEnd := MinInt(length, chunkSize)
	eg, eCtx := errgroup.WithContext(ctx)
	for {
		if err := eCtx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		// Wrap our chunk variables in a closure to keep them from mutating as the loop progresses.
		func(chunkStart, chunkEnd int) {
			eg.Go(func() error {
				err := fn(eCtx, chunkStart, chunkEnd)
				return skerr.Wrapf(err, "chunk[%d:%d]", chunkStart, chunkEnd)
			})
		}(chunkStart, chunkEnd)
		if chunkEnd == length {
			return eg.Wait()
		}
		chunkStart = chunkEnd
		chunkEnd = MinInt(length, chunkEnd+chunkSize)
	}
}

// chunk is used in ChunkIterParallelPool.
type chunk struct {
	start int
	end   int
}

// ChunkIterParallelPool is similar to ChunkIterParallel but it uses a poolSize
// group of workers to run the chunks in parallel.
func ChunkIterParallelPool(ctx context.Context, length, chunkSize, poolSize int, fn func(ctx context.Context, startIdx int, endIdx int) error) error {
	if chunkSize < 1 {
		return skerr.Fmt("chunk size may not be less than 1 (saw %d)", chunkSize)
	}
	if length < 0 {
		return skerr.Fmt("length cannot be negative (saw %d)", length)
	}
	if poolSize < 1 {
		return skerr.Fmt("pool size may not be less than 1 (saw %d)", poolSize)
	}

	// We make the channel buffered with enough space to cover all the chunks,
	// which avoids a deadlock if the context errors (or times out) before we
	// finish putting all the chunks into the channel.
	chunkChannel := make(chan chunk, length/chunkSize+1)

	g, eCtx := errgroup.WithContext(ctx)
	// Start the pool of workers.
	for i := 0; i < poolSize; i++ {
		g.Go(func() error {
			for chunk := range chunkChannel {
				if err := eCtx.Err(); err != nil {
					return skerr.Wrap(err)
				}
				if err := fn(ctx, chunk.start, chunk.end); err != nil {
					// Drain the channel before returning the err.
					for range chunkChannel {
					}
					return err
				}
			}
			return nil
		})
	}

	chunkStart := 0
	chunkEnd := MinInt(length, chunkSize)

	for {
		chunkChannel <- chunk{start: chunkStart, end: chunkEnd}
		if chunkEnd == length {
			break
		}
		chunkStart = chunkEnd
		chunkEnd = MinInt(length, chunkEnd+chunkSize)
	}
	close(chunkChannel)
	return skerr.Wrap(g.Wait())
}

// IsDirEmpty checks to see if the specified directory has any contents.
func IsDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer Close(f)

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// ValidateCommit returns true iff the given commit hash looks valid. Does not
// perform any check as to whether the commit means anything in a particular
// repository.
func ValidateCommit(hash string) bool {
	if len(hash) != 40 {
		return false
	}
	for _, char := range hash {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

// ParseIntSet parses a string expression like "5", "3-8", or "3,4,9" into a
// slice of integers: [5], [3, 4, 5, 6, 7, 8], [3, 4, 9].
func ParseIntSet(expr string) ([]int, error) {
	rv := []int{}
	if expr == "" {
		return rv, nil
	}
	ranges := strings.Split(expr, ",")
	for _, r := range ranges {
		endpoints := strings.Split(r, "-")
		if len(endpoints) == 1 {
			v, err := strconv.Atoi(endpoints[0])
			if err != nil {
				return nil, err
			}
			rv = append(rv, v)
		} else if len(endpoints) == 2 {
			if endpoints[0] == "" {
				return nil, skerr.Fmt("Invalid expression %q", r)
			}
			start, err := strconv.Atoi(endpoints[0])
			if err != nil {
				return nil, err
			}
			if endpoints[1] == "" {
				return nil, skerr.Fmt("Invalid expression %q", r)
			}
			end, err := strconv.Atoi(endpoints[1])
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, skerr.Fmt("Cannot have a range whose beginning is greater than its end (%d vs %d)", start, end)
			}
			for i := start; i <= end; i++ {
				rv = append(rv, i)
			}
		} else {
			return nil, skerr.Fmt("Invalid expression %q", r)
		}
	}
	return rv, nil
}

// ToDos performs like the "todos" tool on Linux: it converts line endings in
// the given string from Unix to Dos format.
func ToDos(s string) string {
	// Run through FromDos first, so that we don't accidentally convert \r\n
	// to \r\r\n.
	return strings.Replace(FromDos(s), "\n", "\r\n", -1)
}

// FromDos performs like the "fromdos" tool on Linux: it converts line endings
// in the given string from Dos to Unix format.
func FromDos(s string) string {
	return strings.Replace(s, "\r\n", "\n", -1)
}

// Truncate the given string to the given length. If the string was shortened,
// change the last three characters to ellipses, unless the specified length is
// 3 or less.
func Truncate(s string, length int) string {
	if len(s) > length {
		if length <= 3 {
			return s[:length]
		}
		ellipses := "..."
		return s[:length-len(ellipses)] + ellipses
	}
	return s
}

// TruncateNoEllipses truncates the given string to the given length, without
// the use of ellipses.
func TruncateNoEllipses(s string, length int) string {
	if len(s) > length {
		return s[:length]
	}
	return s
}

// WithWriteFile provides an interface for writing to a backing file using a
// temporary intermediate file for more atomicity in case a long-running write
// gets interrupted.
func WithWriteFile(file string, writeFn func(io.Writer) error) error {
	if file == "" {
		return skerr.Fmt("file must not be empty")
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return skerr.Wrapf(err, "calling MkdirAll(%s, 0700)", dir)
	}

	tempBase := filepath.Base(file)
	f, err := os.CreateTemp(dir, tempBase)
	if err != nil {
		return skerr.Wrapf(err, "creating temporary file %q in WithWriteFile for %q", tempBase, file)
	}
	if err := writeFn(f); err != nil {
		_ = f.Close() // Ignore the error since we've already failed.
		Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		Remove(f.Name())
		return skerr.Wrapf(err, "closing temporary file in WithWriteFile")
	}
	if err := os.Rename(f.Name(), file); err != nil {
		return skerr.Wrapf(err, "renaming temporary file %s to %s in WithWriteFile", f.Name(), file)
	}
	return nil
}

// WithGzipWriter is a helper for wrapping an io.Writer with a gzip.Writer.
func WithGzipWriter(w io.Writer, fn func(w io.Writer) error) (err error) {
	gzw := gzip.NewWriter(w)
	defer func() {
		err2 := gzw.Close()
		if err == nil && err2 != nil {
			err = skerr.Wrapf(err2, "closing gzip.Writer")
		}
	}()
	err = fn(gzw)
	return
}

// WithReadFile opens the given file for reading and runs the given function.
func WithReadFile(file string, fn func(f io.Reader) error) (err error) {
	var f *os.File
	f, err = os.Open(file)
	if err != nil {
		return
	}
	defer func() {
		err2 := f.Close()
		if err == nil && err2 != nil {
			err = skerr.Wrapf(err2, "closing file %s", file)
		}
	}()
	err = fn(f)
	return
}

// ReadGobFile reads data from the given file into the given data structure.
func ReadGobFile(file string, data interface{}) error {
	return WithReadFile(file, func(f io.Reader) error {
		return gob.NewDecoder(f).Decode(data)
	})
}

// MaybeReadGobFile reads data from the given file into the given data
// structure. If the file does not exist, no error is returned and no data is
// written.
func MaybeReadGobFile(file string, data interface{}) error {
	if err := ReadGobFile(file, data); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// WriteGobFile writes the given data to the given file, using gob encoding.
func WriteGobFile(file string, data interface{}) error {
	return WithWriteFile(file, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(data)
	})
}

// CopyFile copies the given src file to dst.
func CopyFile(src, dst string) (rvErr error) {
	fi, err := os.Stat(src)
	if err != nil {
		return skerr.Wrap(err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer func() {
		if err := srcFile.Close(); err != nil && rvErr != nil {
			rvErr = err
		}
	}()
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, fi.Mode().Perm())
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		// Ignore the error from Close() since we've already failed.
		_ = dstFile.Close()
		return skerr.Wrap(err)
	}
	if err := dstFile.Close(); err != nil {
		return skerr.Wrap(err)
	}
	// We already specified permissions in the call to OpenFile, but we
	// chmod again to work around the umask and ensure that we get the
	// correct permissions.
	return skerr.Wrap(os.Chmod(dst, fi.Mode()))
}

// IterTimeChunks calls the given function for each time chunk of the given
// duration within the given time range.
func IterTimeChunks(start, end time.Time, chunkSize time.Duration, fn func(time.Time, time.Time) error) error {
	chunkStart := start
	for chunkStart.Before(end) {
		chunkEnd := chunkStart.Add(chunkSize)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		if err := fn(chunkStart, chunkEnd); err != nil {
			return err
		}
		chunkStart = chunkEnd
	}
	return nil
}

// Validator is an interface which has a Validate() method.
type Validator interface {
	Validate() error
}

// MultiWriter is like io.MultiWriter but attempts to write to all of the given
// io.Writers, even if writing to one fails.
type MultiWriter []io.Writer

// See documentation for io.Writer. Uses a multierror.Error to summarize any and
// all errors returned by each of the io.Writers.
func (mw MultiWriter) Write(b []byte) (int, error) {
	var rv int
	var rvErr *multierror.Error
	for _, w := range mw {
		n, err := w.Write(b)
		if err != nil {
			rvErr = multierror.Append(rvErr, err)
		} else {
			rv = n
		}
	}
	// Note: multierror.Error.ErrorOrNil() checks whether the instance is
	// nil, so it's safe to call rvErr.ErrorOrNil() even if no error
	// occurred.
	return rv, rvErr.ErrorOrNil()
}

// RoundUpToPowerOf2 rounds the given int up to the nearest power of 2.
func RoundUpToPowerOf2(i int32) int32 {
	// Taken from https://web.archive.org/web/20160703165415/https://graphics.stanford.edu/~seander/bithacks.html#RoundUpPowerOf2
	// Attributed to Sean Anderson.
	if i == 0 {
		return 1
	}
	i--
	i |= i >> 1
	i |= i >> 2
	i |= i >> 4
	i |= i >> 8
	i |= i >> 16
	i++
	return i
}

// AskForConfirmation waits for the user to type "y" or "n".
func AskForConfirmation(format string, args ...interface{}) (bool, error) {
	fmt.Println(fmt.Sprintf(format, args...))
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false, err
	}
	if response == "y" {
		return true, nil
	} else if response == "n" {
		return false, nil
	} else {
		fmt.Println("Please type 'y' or 'n' and then press enter.")
		return AskForConfirmation(format, args...)
	}
}

// PowerSet returns a slice of slices representing the power set of the indices
// of a slice.
func PowerSet(n int) [][]int {
	if n == 0 {
		return [][]int{{}}
	}
	subs := PowerSet(n - 1)
	addl := [][]int{}
	for _, sub := range subs {
		cpy := make([]int, len(sub)+1)
		copy(cpy, sub)
		cpy[len(cpy)-1] = n - 1
		addl = append(addl, cpy)
	}
	return append(subs, addl...)
}

// SSliceDedup deduplicates a slice of strings, preserving their order.
func SSliceDedup(slice []string) []string {
	deduped := []string{}
	seen := map[string]bool{}
	for _, s := range slice {
		if _, ok := seen[s]; !ok {
			seen[s] = true
			deduped = append(deduped, s)
		}
	}
	return deduped
}

// IsLocal attempts to determine whether or not we're running on a developer
// machine vs in Swarming or Kubernetes.
func IsLocal() bool {
	// Check the --local flag.
	localFlag := flag.Lookup("local")
	if localFlag != nil {
		return localFlag.Value.String() == "true"
	}

	// Note: we could also check environment variables we know are present
	// in Swarming and in Kubernetes, but those would have no effect because
	// the default is false.
	return false
}

// FirstNonEmpty returns the first of its args that is not "". It is useful when a certain value
// would be preferred if present but others are available as fallbacks. If all of its args are "",
// returns "".
func FirstNonEmpty(args ...string) string {
	a := ""
	for _, a = range args {
		if a != "" {
			return a
		}
	}
	return a
}

// SplitLines returns a slice of the lines of s, split on newline characters. If the input string
// ends in a single newline, we strip it rather than returning a blank extra line.
//
// Note that this currently works only with UNIX-style line breaks, not DOS \r\n ones, though
// support for those may be added later.
func SplitLines(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

var wordWrapChars = map[rune]bool{
	' ':  true,
	'\t': true,
	'\r': true,
	'\v': true,
}

const wordWrapCharsCutset = " \t\r\v"

func trimWhitespaceFromEnd(s []rune) string {
	return strings.TrimRight(string(s), wordWrapCharsCutset)
}

// WordWrap inserts newlines into the string so that no lone is longer than the
// given lineLength. Only breaks on word boundaries. Strips whitespace from the
// ends of lines but preserves whitespace at the beginnings of lines.
func WordWrap(s string, lineLength int) string {
	inpLines := strings.Split(s, "\n")
	rvLines := make([]string, 0, len(inpLines))
	for _, lineStr := range inpLines {
		line := []rune(lineStr)
		for {
			// Are we done?
			if len(line) <= lineLength {
				rvLines = append(rvLines, trimWhitespaceFromEnd(line))
				break
			}

			// Find whitespace locations where we could split the line.
			splitPoints := []int{}
			foundNonSpaceChar := false
			for idx, char := range line {
				// Have we reached a possible split point?
				if wordWrapChars[char] {
					if foundNonSpaceChar {
						splitPoints = append(splitPoints, idx)
					}
				} else {
					foundNonSpaceChar = true
				}
			}

			// If we found no split points, we're done with this line.
			if len(splitPoints) == 0 {
				rvLines = append(rvLines, trimWhitespaceFromEnd(line))
				break
			}

			// Break the line, continue processing the rest of the line.
			splitPoint := -1
			for _, sp := range splitPoints {
				if sp <= lineLength {
					splitPoint = sp
				} else if splitPoint < 0 {
					splitPoint = sp
				} else {
					break
				}
			}
			rvLines = append(rvLines, trimWhitespaceFromEnd(line[:splitPoint]))
			line = line[splitPoint:]
			for wordWrapChars[line[0]] {
				// Consume the space we broke on.
				line = line[1:]
			}
		}
	}
	return strings.Join(rvLines, "\n")
}
