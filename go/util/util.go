package util

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	mathrand "math/rand"
	"os"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/zeebo/bencode"
	"go.skia.org/infra/go/sklog"
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
	BUGS_PATTERN        = "(?m)^(?:BUG=|Bug:)(.+)$"

	SECONDS_TO_MILLIS = int64(time.Second / time.Millisecond)
	MILLIS_TO_NANOS   = int64(time.Millisecond / time.Nanosecond)
	MICROS_TO_NANOS   = int64(time.Microsecond / time.Nanosecond)

	// time.RFC3339Nano only uses as many sub-second digits are required to
	// represent the time, which makes it unsuitable for sorting. This
	// format ensures that all 9 nanosecond digits are used, padding with
	// zeroes if necessary.
	RFC3339NanoZeroPad = sklog.RFC3339NanoZeroPad

	// SAFE_TIMESTAMP_FORMAT is time format which is similar to
	// RFC3339NanoZeroPad, but with most of the punctuation omitted. This
	// timestamp can only be used to format and parse times in UTC.
	SAFE_TIMESTAMP_FORMAT = "20060102T150405.000000000Z"
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

// Trunc returns s truncated to n+3 chars.
//
// If len(s) < n then the string is unchanged.
func Trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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

// InsertString inserts the given string into the slice at the given index.
func InsertString(strs []string, idx int, s string) []string {
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
		return InsertString(strs, idx, s)
	}
	return strs
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

// ContainsMap checks if child map is contained within the parent map
func ContainsMap(parent, child map[string]string) bool {
	if len(child) > len(parent) {
		return false
	}
	// Since we know child is less than or equal to parent we only need to
	// compare child's values to parent's values.
	for k, v := range child {
		if pv, ok := parent[k]; !ok || pv != v {
			return false
		}
	}
	return true
}

// ContainsAnyMap checks to see if any of the children maps are contained in
// the parent map.
func ContainsAnyMap(parent map[string]string, children ...map[string]string) bool {
	for _, child := range children {
		if ContainsMap(parent, child) {
			return true
		}
	}
	return false
}

// ContainsMapInSliceValues checks if child map is contained within the
// parent map.
func ContainsMapInSliceValues(parent map[string][]string, child map[string]string) bool {
	if len(child) > len(parent) {
		return false
	}
	// Since we know child is less than or equal to parent we only need to
	// compare child's values to parent's values.
	for k, v := range child {
		if pv, ok := parent[k]; !ok || !In(v, pv) {
			return false
		}
	}
	return true
}

// ContainsAnyMapInSliceValues checks to see if any of the children maps are
// contained in the parent map.
func ContainsAnyMapInSliceValues(parent map[string][]string, children ...map[string]string) bool {
	for _, child := range children {
		if ContainsMapInSliceValues(parent, child) {
			return true
		}
	}
	return false
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
	return TimeStamp(time.Millisecond)
}

// Returns the current time in the units defined by the given target unit.
// e.g. TimeStamp(time.Millisecond) will return the time in Milliseconds.
// The result is always rounded down to the lowest integer from the
// representation in nano seconds.
func TimeStamp(targetUnit time.Duration) int64 {
	return time.Now().UnixNano() / int64(targetUnit)
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

// Rename renames the specified file and logs an error if one is returned.
func Rename(oldpath, newpath string) {
	if err := os.Rename(oldpath, newpath); err != nil {
		sklog.ErrorfWithDepth(1, "Failed to Rename(%s, %s): %v", oldpath, newpath, err)
	}
}

// Mkdir creates the specified path and logs an error if one is returned.
func Mkdir(name string, perm os.FileMode) {
	if err := os.Mkdir(name, perm); err != nil {
		sklog.ErrorfWithDepth(1, "Failed to Mkdir(%s, %v): %v", name, perm, err)
	}
}

// MkdirAll creates the specified path and logs an error if one is returned.
func MkdirAll(name string, perm os.FileMode) {
	if err := os.MkdirAll(name, perm); err != nil {
		sklog.ErrorfWithDepth(1, "Failed to MkdirAll(%s, %v): %v", name, perm, err)
	}
}

// LogErr logs err if it's not nil. This is intended to be used
// for calls where generally a returned error can be ignored.
func LogErr(err error) {
	if err != nil {
		sklog.ErrorfWithDepth(1, "Unexpected error: %s", err)
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

func ParseTimeNs(t string) (time.Time, error) {
	i, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, i).UTC(), nil
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
func RepeatCtx(interval time.Duration, ctx context.Context, fn func()) {
	ticker := time.NewTicker(interval)
	done := ctx.Done()
	defer ticker.Stop()
	fn()
MainLoop:
	for {
		select {
		case <-done:
			break MainLoop
		case <-ticker.C:
			fn()
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
func ChunkIter(length, chunkSize int, fn func(int, int) error) error {
	if chunkSize < 1 {
		return fmt.Errorf("Chunk size may not be less than 1.")
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

// ChunkIterInt iterates over a slice of ints in chunks of smaller slices.
func ChunkIterInt(s []int, chunkSize int, fn func([]int) error) error {
	return ChunkIter(len(s), chunkSize, func(start, end int) error {
		return fn(s[start:end])
	})
}

// BugsFromCommitMsg parses BUG= tags from a commit message and returns them.
func BugsFromCommitMsg(msg string) map[string][]string {
	rv := map[string][]string{}
	m := BUGS_REGEX.FindAllStringSubmatch(msg, -1)
	for _, match := range m {
		for _, s := range match[1:] {
			bugs := strings.Split(s, ",")
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
	}
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

// CookieDomainMatch returns True if domainA domain-matches domainB, according to RFC 2965.
// domainA and domainB may only be host domain names. IP addresses are currently not supported.
//
// RFC 2965, section 1:
//   Host names can be specified either as an IP address or a HDN string.
//   Sometimes we compare one host name with another.  (Such comparisons SHALL
//   be case-insensitive.)  Host A's name domain-matches host B's if
//   * their host name strings string-compare equal; or
//   * A is a HDN string and has the form NB, where N is a non-empty
//     name string, B has the form .B', and B' is a HDN string.  (So,
//     x.y.com domain-matches .Y.com but not Y.com.)
//   Note that domain-match is not a commutative operation: a.b.c.com
//   domain-matches .c.com, but not the reverse.
func CookieDomainMatch(domainA, domainB string) bool {
	a := strings.ToLower(domainA)
	b := strings.ToLower(domainB)
	initialDot := strings.HasPrefix(b, ".")
	if initialDot && strings.HasSuffix(a, b) {
		return true
	}
	if !initialDot && a == b {
		return true
	}
	return false
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

// Permute returns all permutations of the given slice of ints. Duplicates in
// the input slice will result in duplicate permutations returned.
func Permute(ints []int) [][]int {
	if len(ints) == 1 {
		return [][]int{{ints[0]}}
	}
	rv := [][]int{}
	for _, i := range ints {
		remaining := make([]int, 0, len(ints)-1)
		for _, j := range ints {
			if j != i {
				remaining = append(remaining, j)
			}
		}
		got := Permute(remaining)
		for _, list := range got {
			// TODO(borenet): These temporary lists are expensive.
			// If we need this to be performant, we should re-
			// implement without copies.
			rv = append(rv, append([]int{i}, list...))
		}
	}
	return rv
}

// PermuteStrings returns all permutations of the given slice of strings.
// Duplicates in the input slice will result in duplicate permutations returned.
func PermuteStrings(strs []string) [][]string {
	idxs := make([]int, 0, len(strs))
	for i := range strs {
		idxs = append(idxs, i)
	}
	permuteIdxs := Permute(idxs)
	rv := make([][]string, 0, len(permuteIdxs))
	for _, idxPerm := range permuteIdxs {
		strPerm := make([]string, 0, len(idxPerm))
		for _, idx := range idxPerm {
			strPerm = append(strPerm, strs[idx])
		}
		rv = append(rv, strPerm)
	}
	return rv
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
				return nil, fmt.Errorf("Invalid expression %q", r)
			}
			start, err := strconv.Atoi(endpoints[0])
			if err != nil {
				return nil, err
			}
			if endpoints[1] == "" {
				return nil, fmt.Errorf("Invalid expression %q", r)
			}
			end, err := strconv.Atoi(endpoints[1])
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, fmt.Errorf("Cannot have a range whose beginning is greater than its end (%d vs %d)", start, end)
			}
			for i := start; i <= end; i++ {
				rv = append(rv, i)
			}
		} else {
			return nil, fmt.Errorf("Invalid expression %q", r)
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

// WithWriteFile provides an interface for writing to a backing file using a
// temporary intermediate file for more atomicity in case a long-running write
// gets interrupted.
func WithWriteFile(file string, writeFn func(io.Writer) error) error {
	f, err := ioutil.TempFile(path.Dir(file), path.Base(file))
	if err != nil {
		return fmt.Errorf("Failed to create temporary file for WithWriteFile: %s", err)
	}
	if err := writeFn(f); err != nil {
		Close(f)
		Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		Remove(f.Name())
		return fmt.Errorf("Failed to close temporary file for WithWriteFile: %s", err)
	}
	if err := os.Rename(f.Name(), file); err != nil {
		return fmt.Errorf("Failed to rename temporary file for WithWriteFile: %s", err)
	}
	return nil
}

// WithBufferedWriter is a helper for wrapping an io.Writer with a bufio.Writer.
func WithBufferedWriter(w io.Writer, fn func(w io.Writer) error) (err error) {
	buf := bufio.NewWriter(w)
	if err := fn(buf); err != nil {
		return err
	}
	return buf.Flush()
}

// WithGzipWriter is a helper for wrapping an io.Writer with a gzip.Writer.
func WithGzipWriter(w io.Writer, fn func(w io.Writer) error) (err error) {
	gzw := gzip.NewWriter(w)
	defer func() {
		err2 := gzw.Close()
		if err == nil && err2 != nil {
			err = fmt.Errorf("Failed to close gzip.Writer: %s", err2)
		}
	}()
	err = fn(gzw)
	return
}

// WithReadFile opens the given file for reading and runs the given function.
func WithReadFile(file string, fn func(f io.Reader) error) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer Close(f)
	return fn(f)
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

// SafeParseInt parses a string that is known to contain digits into an int.
// If the number is larger than MAX_INT, 0 will be returned after
// logging an error.
func SafeAtoi(n string) int {
	if i, err := strconv.Atoi(n); err != nil {
		sklog.Errorf("Could not parse number from known digits %q: %v", n, err)
		return 0
	} else {
		return i
	}
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

// ThreadSafeWriter wraps an io.Writer and provides thread safety.
type ThreadSafeWriter struct {
	w   io.Writer
	mtx sync.Mutex
}

// See documentation for io.Writer.
func (w *ThreadSafeWriter) Write(b []byte) (int, error) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	return w.w.Write(b)
}

// NewThreadSafeWriter returns a ThreadSafeWriter which wraps the given Writer.
func NewThreadSafeWriter(w io.Writer) io.Writer {
	return &ThreadSafeWriter{
		w: w,
	}
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

// SSliceCmp compares two string slices by comparing each element in order.
// Returns -1 if the first slice is "less" than the second, 0 if they are equal,
// and 1 if the first slice is "greater" than the second.
func SSliceCmp(a, b []string) int {
	for i, elemA := range a {
		if len(b) <= i {
			// If slice B is shorter than slice A, then A is not
			// less than B.
			return 1
		}
		elemB := b[i]
		if elemA < elemB {
			return -1
		} else if elemA > elemB {
			return 1
		}
	}
	// The two slices are equal, up to len(a). If the lengths are the same,
	// then the slices are equal. Otherwise, a < b.
	if len(a) == len(b) {
		return 0
	}
	return -1
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
