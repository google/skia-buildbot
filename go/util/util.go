package util

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	mathrand "math/rand"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"github.com/zeebo/bencode"
)

const (
	_          = iota // ignore first value by assigning to blank identifier
	KB float64 = 1 << (10 * iota)
	MB
	GB
	TB
	PB

	PROJECT_CHROMIUM    = "chromium"
	BUG_DEFAULT_PROJECT = PROJECT_CHROMIUM
	BUGS_PATTERN        = "(?m)^BUG=(.+)$"

	SECONDS_TO_MILLIS = int64(time.Second / time.Millisecond)
	MILLIS_TO_NANOS   = int64(time.Millisecond / time.Nanosecond)

	// time.RFC3339Nano only uses as many sub-second digits are required to
	// represent the time, which makes it unsuitable for sorting. This
	// format ensures that all 9 nanosecond digits are used, padding with
	// zeroes if necessary.
	RFC3339NanoZeroPad = "2006-01-02T15:04:05.000000000Z07:00"
)

var (
	BUGS_REGEX = regexp.MustCompile(BUGS_PATTERN)

	// randomNameAdj is a list of adjectives for building random names.
	randomNameAdj = []string{
		"autumn", "hidden", "bitter", "misty", "silent", "empty", "dry", "dark",
		"summer", "icy", "delicate", "quiet", "white", "cool", "spring", "winter",
		"patient", "twilight", "dawn", "crimson", "wispy", "weathered", "blue",
		"billowing", "broken", "cold", "damp", "falling", "frosty", "green",
		"long", "late", "lingering", "bold", "little", "morning", "muddy", "old",
		"red", "rough", "still", "small", "sparkling", "throbbing", "shy",
		"wandering", "withered", "wild", "black", "young", "holy", "solitary",
		"fragrant", "aged", "snowy", "proud", "floral", "restless", "divine",
		"polished", "ancient", "purple", "lively", "nameless",
	}

	// randomNameNoun is a list of nouns for building random names.
	randomNameNoun = []string{
		"waterfall", "river", "breeze", "moon", "rain", "wind", "sea", "morning",
		"snow", "lake", "sunset", "pine", "shadow", "leaf", "dawn", "glitter",
		"forest", "hill", "cloud", "meadow", "sun", "glade", "bird", "brook",
		"butterfly", "bush", "dew", "dust", "field", "fire", "flower", "firefly",
		"feather", "grass", "haze", "mountain", "night", "pond", "darkness",
		"snowflake", "silence", "sound", "sky", "shape", "surf", "thunder",
		"violet", "water", "wildflower", "wave", "water", "resonance", "sun",
		"wood", "dream", "cherry", "tree", "fog", "frost", "voice", "paper",
		"frog", "smoke", "star",
	}

	TimeZero     = time.Time{}.UTC()
	TimeUnixZero = time.Unix(0, 0).UTC()
)

// GetFormattedByteSize returns a formatted pretty string representation of the
// provided byte size. Eg: Input of 1024 would return "1.00KB".
func GetFormattedByteSize(b float64) string {
	switch {
	case b >= PB:
		return fmt.Sprintf("%.2fPB", b/PB)
	case b >= TB:
		return fmt.Sprintf("%.2fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}
	return fmt.Sprintf("%.2fB", b)
}

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

// AtMost returns a subslice of at most the first n members of a.
func AtMost(a []string, n int) []string {
	if n > len(a) {
		n = len(a)
	}
	return a[:n]
}

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
	sort.Strings(a)
	sort.Strings(b)
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

type Int64Slice []int64

func (p Int64Slice) Len() int           { return len(p) }
func (p Int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Int64Equal returns true if the int64 slices are equal.
func Int64Equal(a, b []int64) bool {
	sort.Sort(Int64Slice(a))
	sort.Sort(Int64Slice(b))
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if x != b[i] {
			return false
		}
	}
	return true
}

// MapsEqual checks if the two maps are equal.
func MapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	// Since they are the same size we only need to check from one side, i.e.
	// compare a's values to b's values.
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

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

// AbsInt returns the absolute value of v.
func AbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// SignInt returns -1, 1 or 0 depending on the sign of v.
func SignInt(v int) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

// Returns the current time in milliseconds since the epoch.
func TimeStampMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// Generate a 16-byte random ID.
func GenerateID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%X", b), nil
}

// IntersectIntSets calculates the intersection of a list
// of integer sets.
func IntersectIntSets(sets []map[int]bool, minIdx int) map[int]bool {
	resultSet := make(map[int]bool, len(sets[minIdx]))
	for val := range sets[minIdx] {
		resultSet[val] = true
	}

	for _, oneSet := range sets {
		for k := range resultSet {
			if !oneSet[k] {
				delete(resultSet, k)
			}
		}
	}

	return resultSet
}

// KeysOfIntSet returns the keys of a set of strings represented by the keys
// of a map.
func KeysOfIntSet(set map[int]bool) []int {
	ret := make([]int, 0, len(set))
	for v := range set {
		ret = append(ret, v)
	}
	return ret
}

// RepeatJoin repeats a given string N times with the given separator between
// each instance.
func RepeatJoin(str, sep string, n int) string {
	if n <= 0 {
		return ""
	}
	return str + strings.Repeat(sep+str, n-1)
}

func AddParamsToParamSet(a map[string][]string, b map[string]string) map[string][]string {
	for k, v := range b {
		// You might be tempted to replace this with
		// sort.SearchStrings(), but that's actually slower for short
		// slices. The breakpoint seems to around 50, and since most
		// of our ParamSet lists are short that ends up being slower.
		if _, ok := a[k]; !ok {
			a[k] = []string{v}
		} else if !In(v, a[k]) {
			a[k] = append(a[k], v)
		}
	}
	return a
}

func AddParamSetToParamSet(a map[string][]string, b map[string][]string) map[string][]string {
	for k, arr := range b {
		for _, v := range arr {
			if _, ok := a[k]; !ok {
				a[k] = []string{v}
			} else if !In(v, a[k]) {
				a[k] = append(a[k], v)
			}
		}
	}
	return a
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

// CopyStringMap returns a copy of the provided map[string]string.
func CopyStringMap(m map[string]string) map[string]string {
	ret := make(map[string]string, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

// KeysOfParamSet returns the keys of a param set.
func KeysOfParamSet(set map[string][]string) []string {
	ret := make([]string, 0, len(set))
	for v := range set {
		ret = append(ret, v)
	}

	return ret
}

// Close wraps an io.Closer and logs an error if one is returned.
func Close(c io.Closer) {
	if err := c.Close(); err != nil {
		glog.Errorf("Failed to Close(): %v", err)
	}
}

// RemoveAll removes the specified path and logs an error if one is returned.
func RemoveAll(path string) {
	if err := os.RemoveAll(path); err != nil {
		glog.Errorf("Failed to RemoveAll(%s): %v", path, err)
	}
}

// Remove removes the specified file and logs an error if one is returned.
func Remove(name string) {
	if err := os.Remove(name); err != nil {
		glog.Errorf("Failed to Remove(%s): %v", name, err)
	}
}

// Rename renames the specified file and logs an error if one is returned.
func Rename(oldpath, newpath string) {
	if err := os.Rename(oldpath, newpath); err != nil {
		glog.Errorf("Failed to Rename(%s, %s): %v", oldpath, newpath, err)
	}
}

// Mkdir creates the specified path and logs an error if one is returned.
func Mkdir(name string, perm os.FileMode) {
	if err := os.Mkdir(name, perm); err != nil {
		glog.Errorf("Failed to Mkdir(%s, %v): %v", name, perm, err)
	}
}

// MkdirAll creates the specified path and logs an error if one is returned.
func MkdirAll(name string, perm os.FileMode) {
	if err := os.MkdirAll(name, perm); err != nil {
		glog.Errorf("Failed to MkdirAll(%s, %v): %v", name, perm, err)
	}
}

// LogErr logs err if it's not nil. This is intended to be used
// for calls where generally a returned error can be ignored.
func LogErr(err error) {
	if err != nil {
		errMsg := ""
		if _, fileName, line, ok := runtime.Caller(1); ok {
			errMsg = fmt.Sprintf("-called from: %s:%d", fileName, line)
		}

		glog.Errorf("Unexpected error %s: %s", errMsg, err)
	}
}

// GetStackTrace returns the stacktrace including GetStackTrace itself.
func GetStackTrace() string {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	return string(buf)
}

// RandomName returns a randomly-generated name of the form, "adjective-noun-number",
// using the default generator from the math/rand package.
func RandomName() string {
	return RandomNameR(nil)
}

// RandomNameR returns a randomly-generated name of the form, "adjective-noun-number",
// using the given math/rand.Rand instance.
func RandomNameR(r *mathrand.Rand) string {
	a := 0
	n := 0
	suffix := 0
	if r == nil {
		a = mathrand.Intn(len(randomNameAdj))
		n = mathrand.Intn(len(randomNameNoun))
		suffix = mathrand.Intn(1000000)
	} else {
		a = r.Intn(len(randomNameAdj))
		n = r.Intn(len(randomNameNoun))
		suffix = r.Intn(1000000)
	}
	return fmt.Sprintf("%s-%s-%d", randomNameAdj[a], randomNameNoun[n], suffix)
}

// StringToCodeName returns a name generated from the source string. The string
// is hashed and used as the seed for a random number generator.
func StringToCodeName(s string) string {
	sum := sha256.Sum256([]byte(s))
	seed := int64(sum[0])<<56 | int64(sum[1])<<48 | int64(sum[2])<<40 | int64(sum[3])<<32 | int64(sum[4])<<24 | int64(sum[5])<<16 | int64(sum[6])<<8 | int64(sum[7])
	r := mathrand.New(mathrand.NewSource(seed))
	return RandomNameR(r)
}

// Float64StableSum returns the sum of the elements of the given []float64
// in a relatively stable manner.
func Float64StableSum(s []float64) float64 {
	sort.Sort(sort.Float64Slice(s))
	sum := 0.0
	for _, elem := range s {
		sum += elem
	}
	return sum
}

// AnyMatch returns true iff the given string matches any regexp in the slice.
func AnyMatch(re []*regexp.Regexp, s string) bool {
	for _, r := range re {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}

// Returns true if i is nil or is an interface containing a nil or invalid value.
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

// MD5Params returns the MD5 hash of the provided map[string]string.
// This could easily be extended to support hashing any datatype supported
// by bencode.
func MD5Params(val map[string]string) (string, error) {
	md5Writer := md5.New()
	enc := bencode.NewEncoder(md5Writer)
	if err := enc.Encode(val); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5Writer.Sum(nil)), nil
}

// Round rounds the given float64 to the nearest whole integer.
func Round(v float64) float64 {
	return math.Floor(v + float64(0.5))
}

// UnixFloatToTime takes a float64 representing a Unix timestamp in seconds and
// returns a time.Time. Rounds to milliseconds.
func UnixFloatToTime(t float64) time.Time {
	roundMillis := int64(Round(t * float64(SECONDS_TO_MILLIS)))
	secs := roundMillis / SECONDS_TO_MILLIS
	millis := roundMillis - (secs * SECONDS_TO_MILLIS)
	nanos := millis * MILLIS_TO_NANOS
	return time.Unix(secs, nanos)
}

// TimeToUnixFloat takes a time.Time and returns a float64 representing a Unix timestamp.
func TimeToUnixFloat(t time.Time) float64 {
	if t.IsZero() {
		return 0.0
	}
	return float64(t.UTC().UnixNano()) / float64(SECONDS_TO_MILLIS*MILLIS_TO_NANOS)
}

// UnixMillisToTime takes an int64 representing a Unix timestamp in milliseconds
// and returns a time.Time.
func UnixMillisToTime(t int64) time.Time {
	return time.Unix(0, t*MILLIS_TO_NANOS).UTC()
}

// TimeIsZero returns true if the time.Time is a zero-value or corresponds to
// a zero Unix timestamp.
func TimeIsZero(t time.Time) bool {
	utc := t.UTC()
	if utc == TimeZero {
		return true
	}
	if utc == TimeUnixZero {
		return true
	}
	return false
}

// Repeat calls the provided function 'fn' immediately and then in intervals
// defined by 'interval'. If anything it sent on the provided stop channel,
// the iteration stops.
func Repeat(interval time.Duration, stopCh <-chan bool, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	fn()
MainLoop:
	for {
		select {
		case <-ticker.C:
			fn()
		case <-stopCh:
			break MainLoop
		}
	}
}

// MD5FromReader returns the MD5 hash of the content in the provided reader.
// If the writer w is not nil it will also write the content of the reader to w.
func MD5FromReader(r io.Reader, w io.Writer) ([]byte, error) {
	hashWriter := md5.New()
	var tempOut io.Writer
	if w == nil {
		tempOut = hashWriter
	} else {
		tempOut = io.MultiWriter(w, hashWriter)
	}

	if _, err := io.Copy(tempOut, r); err != nil {
		return nil, err
	}
	return hashWriter.Sum(nil), nil
}

// ChunkIter iterates over a slice in chunks of smaller slices.
func ChunkIter(s []int, chunkSize int, fn func([]int) error) error {
	if chunkSize < 1 {
		return fmt.Errorf("Chunk size may not be less than 1.")
	}
	for c, r := s[:MinInt(chunkSize, len(s))], s[MinInt(chunkSize, len(s)):]; len(c) > 0; c, r = r[:MinInt(chunkSize, len(r))], r[MinInt(chunkSize, len(r)):] {
		if err := fn(c); err != nil {
			return err
		}
	}
	return nil
}

// BugsFromCommitMsg parses BUG= tags from a commit message and returns them.
func BugsFromCommitMsg(msg string) map[string][]string {
	rv := map[string][]string{}
	m := BUGS_REGEX.FindStringSubmatch(msg)
	if len(m) > 1 {
		bugs := strings.Split(m[1], ",")
		for _, b := range bugs {
			b = strings.Trim(b, " ")
			split := strings.SplitN(strings.Trim(b, " "), ":", 2)
			project := BUG_DEFAULT_PROJECT
			bug := split[0]
			if len(split) > 1 {
				project = split[0]
				bug = split[1]
			}
			if rv[project] == nil {
				rv[project] = []string{}
			}
			rv[project] = append(rv[project], bug)
		}
	}
	glog.Errorf("%v", rv)
	return rv
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
