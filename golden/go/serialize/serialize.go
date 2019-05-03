package serialize

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

// Sample contains the information necessary to represent the full state of
// a Gold instance and a sample from a live instance.
type Sample struct {
	// The JSON key names are set to keep the serializable code able to
	// read old files.
	Tile           *tiling.Tile         `json:"Tile"`
	TestExpBuilder types.TestExpBuilder `json:"Expectations"`
	IgnoreRules    []*ignore.IgnoreRule `json:"IgnoreRules"`
}

// Serialize writes this Sample instance to the given writer.
func (s *Sample) Serialize(w io.Writer) error {
	expBytes, err := json.Marshal(s.TestExpBuilder)
	if err != nil {
		return err
	}
	if err := writeBytesWithLength(w, expBytes); err != nil {
		return err
	}

	ignoreBytes, err := json.Marshal(s.IgnoreRules)
	if err != nil {
		return err
	}
	if err := writeBytesWithLength(w, ignoreBytes); err != nil {
		return err
	}

	if err := SerializeTile(w, s.Tile); err != nil {
		return err
	}
	return nil
}

// DeserializeSample returns a new instance of Sample from the given reader. It
// is the inverse operation of Sample.Searialize.
func DeserializeSample(r io.Reader) (*Sample, error) {
	ret := &Sample{
		TestExpBuilder: types.NewTestExpBuilder(nil),
	}

	expBytes, err := readBytesWithLength(r)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(expBytes, &ret.TestExpBuilder); err != nil {
		return nil, err
	}

	ignoreBytes, err := readBytesWithLength(r)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(ignoreBytes, &ret.IgnoreRules); err != nil {
		return nil, err
	}

	if ret.Tile, err = DeserializeTile(r); err != nil {
		return nil, err
	}

	return ret, nil
}

// UnmarshalJSON allows to deserialize an instance of Sample that has been
// serialized using the json package.
func (s *Sample) UnmarshalJSON(data []byte) error {
	var dummy struct {
		Tile           json.RawMessage      `json:"Tile"`
		TestExpBuilder types.TestExpBuilder `json:"Expectations"`
		IgnoreRules    []*ignore.IgnoreRule `json:"IgnoreRules"`
	}
	var err error

	if err = json.Unmarshal(data, &dummy); err != nil {
		return err
	}

	s.Tile, err = types.TileFromJson(bytes.NewBuffer(dummy.Tile), &types.GoldenTrace{})
	if err != nil {
		return fmt.Errorf("Error decoding tile from raw message: %s", err)
	}

	s.TestExpBuilder = dummy.TestExpBuilder
	s.IgnoreRules = dummy.IgnoreRules
	return nil
}

// SerializeTile writes the tile to the given writer.
func SerializeTile(w io.Writer, tile *tiling.Tile) error {
	if err := writeCommits(w, tile.Commits); err != nil {
		return err
	}

	// Write combined ParamSets
	paramKeyTable, paramValueTable, err := writeParamSets(w, tile.ParamSet)
	if err != nil {
		return err
	}

	// Write digests
	digestTable, err := writeDigests(w, tile.Traces)
	if err != nil {
		return err
	}

	// Serialize the traces with the look up tables created in the
	// previous setp.
	for id, trace := range tile.Traces {
		if err := writeTrace(w, paramKeyTable, paramValueTable, digestTable, id, trace.(*types.GoldenTrace)); err != nil {
			return err
		}
	}

	return nil
}

// DeserializeTile reads the tile from the given reader.
func DeserializeTile(r io.Reader) (*tiling.Tile, error) {
	commits, err := readCommits(r)
	if err != nil {
		return nil, err
	}

	nCommits := len(commits)
	paramKeyTable, paramValTable, err := readParamSets(r)
	if err != nil {
		return nil, err
	}

	digestTable, err := readDigests(r)
	if err != nil {
		return nil, err
	}

	traces := map[string]tiling.Trace{}
	paramSets := paramtools.ParamSet{}
	for {
		id, gTrace, err := readTrace(r, paramKeyTable, paramValTable, digestTable, nCommits)
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}
		traces[id] = gTrace
		paramSets.AddParams(gTrace.Keys)
	}

	ret := &tiling.Tile{
		ParamSet: paramSets,
		Traces:   traces,
		Commits:  commits,
	}

	return ret, nil
}

// CacheTile to disk writes the given tile to the given path.
// It will first write to a temporary file and then rename it
// to target path.
func CacheTile(tile *tiling.Tile, path string) error {
	defer shared.NewMetricsTimer("write_cached_tile").Stop()
	dirName, fileName := filepath.Split(path)

	outFile, err := ioutil.TempFile(dirName, fileName)
	if err != nil {
		return err
	}

	if err := SerializeTile(outFile, tile); err != nil {
		return err
	}

	if err := outFile.Close(); err != nil {
		return err
	}

	if fileutil.FileExists(path) {
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	return os.Rename(outFile.Name(), path)
}

// LoadCachedTile loads the cached tile at the given path.
// If the path does not exist, it will not return an error, but
// the returned tile will be nil.
func LoadCachedTile(path string) (*tiling.Tile, error) {
	if !fileutil.FileExists(path) {
		return nil, nil
	}

	defer shared.NewMetricsTimer("load_cached_tile").Stop()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)

	return DeserializeTile(f)
}

// stringsToBytes converts and array of strings to a byte slice with zero
// terminated  strings.
func stringsToBytes(arr []string) []byte {
	var buffer bytes.Buffer
	for _, element := range arr {
		_, _ = buffer.WriteString(element)
		_ = buffer.WriteByte(0)
	}
	return buffer.Bytes()
}

// bytesToStrings conversts a byte slice with zero terminated strings to a slice
// of strings.
func bytesToStrings(arr []byte) ([]string, error) {
	ret := []string{}
	buffer := bytes.NewBuffer(arr)
	for {
		str, err := buffer.ReadBytes(0)
		if err == nil {
			ret = append(ret, string(str[:len(str)-1]))
		} else if err != io.EOF {
			return nil, err
		} else if len(str) > 0 {
			return nil, fmt.Errorf("Invalid EOF.")
		} else {
			return ret, nil
		}
	}
}

// writeBytesWithLength writes the given byte slice to the given writer,
// prefixed with the length of slice.
func writeBytesWithLength(w io.Writer, byteArr []byte) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(byteArr))); err != nil {
		return err
	}

	n, err := w.Write(byteArr)
	if err != nil {
		return err
	}
	if n != len(byteArr) {
		return fmt.Errorf("Unable to write array of length %d. Only wrote %d bytes.", len(byteArr), n)
	}
	return nil
}

// readBytesWithLength reads the bytes slice from the given reader. The byte
// slice is assumed to be prefixed with its length.
func readBytesWithLength(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, err
	}

	buf := make([]byte, length)
	if nBytesRead, err := r.Read(buf); err != nil {
		return nil, fmt.Errorf("Unable to read required bytes: %s", err)
	} else if uint32(nBytesRead) != length {
		return nil, fmt.Errorf("Expected %d bytes, got %d bytes.", length, nBytesRead)
	}
	return buf, nil
}

// writeStringArr writes the given string slice to disk with its length.
func writeStringArr(w io.Writer, arr []string) error {
	return writeBytesWithLength(w, stringsToBytes(arr))
}

// readStringArr reads an array of strings (prefixed with their length) back
// from the given reader.
func readStringArr(r io.Reader) ([]string, error) {
	byteArr, err := readBytesWithLength(r)
	if err != nil {
		return nil, err
	}
	return bytesToStrings(byteArr)
}

// writeCommits writes the given commits to the writer.
func writeCommits(w io.Writer, commits []*tiling.Commit) error {
	nCommits := len(commits)
	hashes := make([]string, 0, nCommits)
	authors := make([]string, 0, nCommits)

	if err := binary.Write(w, binary.LittleEndian, uint32(nCommits)); err != nil {
		return err
	}

	for _, commit := range commits {
		if err := binary.Write(w, binary.LittleEndian, commit.CommitTime); err != nil {
			return err
		}
		hashes = append(hashes, commit.Hash)
		authors = append(authors, commit.Author)
	}

	if err := writeStringArr(w, hashes); err != nil {
		return err
	}
	if err := writeStringArr(w, authors); err != nil {
		return err
	}
	return nil
}

// readCommits reads commits from the given reader.
func readCommits(r io.Reader) ([]*tiling.Commit, error) {
	var nCommits uint32
	if err := binary.Read(r, binary.LittleEndian, &nCommits); err != nil {
		return nil, err
	}

	times := make([]int64, 0, nCommits)
	var t int64
	for i := uint32(0); i < nCommits; i++ {
		if err := binary.Read(r, binary.LittleEndian, &t); err != nil {
			return nil, err
		}
		times = append(times, t)
	}

	hashes, err := readStringArr(r)
	if err != nil {
		return nil, err
	}

	authors, err := readStringArr(r)
	if err != nil {
		return nil, err
	}

	if (len(times) != len(hashes)) || (len(times) != len(authors)) {
		return nil, fmt.Errorf("Lengths of times, hashes and authors do not match. Got %d != %d != %d", len(times), len(hashes), len(authors))
	}

	ret := make([]*tiling.Commit, 0, nCommits)
	for i, t := range times {
		ret = append(ret, &tiling.Commit{
			CommitTime: t,
			Hash:       hashes[i],
			Author:     authors[i],
		})
	}

	return ret, nil
}

// INT_SIZE is the number of bytes we use for a single number to encode.
const BYTES_PER_INT = 4

// writeParamSets writes the given to the writer and returns mappings to encode
// the keys and values of the underlying params.
func writeParamSets(w io.Writer, paramSets paramtools.ParamSet) (map[string]int, map[string]int, error) {
	paramKeys := make([]string, 0, len(paramSets))
	paramVals := util.NewStringSet()
	for key, values := range paramSets {
		paramKeys = append(paramKeys, key)
		paramVals.AddLists(values)
	}

	if err := writeStringArr(w, paramKeys); err != nil {
		return nil, nil, err
	}

	valList := paramVals.Keys()
	if err := writeStringArr(w, valList); err != nil {
		return nil, nil, err
	}

	keyTable := make(map[string]int, len(paramKeys))
	for idx, key := range paramKeys {
		keyTable[key] = idx
	}

	valTable := make(map[string]int, len(valList))
	for idx, val := range valList {
		valTable[val] = idx
	}

	return keyTable, valTable, nil
}

// readParamSets reads the keys and values that are used to encode a paramset
// from the given reader and returns tables to decode them.
func readParamSets(r io.Reader) (map[int]string, map[int]string, error) {
	keys, err := readStringArr(r)
	if err != nil {
		return nil, nil, err
	}

	vals, err := readStringArr(r)
	if err != nil {
		return nil, nil, err
	}

	keysTable := make(map[int]string, len(keys))
	for idx, key := range keys {
		keysTable[idx] = key
	}

	valsTable := make(map[int]string, len(vals))
	for idx, val := range vals {
		valsTable[idx] = val
	}

	return keysTable, valsTable, nil
}

// paramsToBytes converts the given params to a byte slice using the
// conversion tables for keys and values.
func paramsToBytes(keyTable map[string]int, valTable map[string]int, params map[string]string) []byte {
	kvPairs := make([]int, 0, len(params)*2)
	for k, v := range params {
		kvPairs = append(kvPairs, keyTable[k], valTable[v])
	}
	return intsToBytes(kvPairs)
}

// bytesToParams converts the given byte slice back to paramtools.Params instance
// using the given conversion tables.
func bytesToParams(keyTable map[int]string, valTable map[int]string, arr []byte) (paramtools.Params, error) {
	kvPairs, err := bytesToInts(arr)
	if err != nil {
		return nil, err
	}

	if (len(kvPairs) % 2) != 0 {
		return nil, fmt.Errorf("Number of key/value pairs needs to be even. Got array of size: %d", len(kvPairs))
	}

	ret := paramtools.Params(make(map[string]string, len(kvPairs)/2))
	for i := 0; i < len(kvPairs); i += 2 {
		ret[keyTable[kvPairs[i]]] = valTable[kvPairs[i+1]]
	}

	return ret, nil
}

// intsToBytes convers the given array of ints to a byte slice.
func intsToBytes(arr []int) []byte {
	var buf bytes.Buffer
	for _, i := range arr {
		for j := 0; j < BYTES_PER_INT; j++ {
			_ = buf.WriteByte(byte(i))
			i >>= 8
		}
	}
	return buf.Bytes()
}

// bytesToInts converts the given byte slice to an integer array.
func bytesToInts(arr []byte) ([]int, error) {
	if len(arr)%BYTES_PER_INT != 0 {
		return nil, fmt.Errorf("Size of byte slice is not a multiple of underlying type. Expected %d for type size %d", len(arr), BYTES_PER_INT)
	}
	retLen := len(arr) / BYTES_PER_INT
	ret := make([]int, retLen)
	j := 0
	for i := 0; i < retLen; i++ {
		val := 0
		for k := BYTES_PER_INT - 1; k > 0; k-- {
			val |= int(arr[j+k])
			val <<= 8
		}
		ret[i] = val | int(arr[j])
		j += BYTES_PER_INT
	}
	return ret, nil
}

// writeDigests writes the given traces to disk. They are assumed to be instance of GoldenTrace.
// And returns a mappint table between the digests and integers.
func writeDigests(w io.Writer, traces map[string]tiling.Trace) (map[string]int, error) {
	digestSet := util.NewStringSet()
	for _, trace := range traces {
		digestSet.AddLists(trace.(*types.GoldenTrace).Digests)
	}

	digests := digestSet.Keys()
	if len(digests) > int(math.Pow(2, BYTES_PER_INT*8)) {
		return nil, fmt.Errorf("Not enough bytes to encode digests. %d > %d", len(digests), int(math.Pow(2, BYTES_PER_INT*8)))
	}

	if err := writeStringArr(w, digests); err != nil {
		return nil, err
	}

	digestTable := make(map[string]int, len(digests))
	for idx, d := range digests {
		digestTable[d] = idx
	}

	return digestTable, nil
}

// readDigests reads digests from the given reader and returns a table to
// map between integers and strings.
func readDigests(r io.Reader) (map[int]string, error) {
	digests, err := readStringArr(r)
	if err != nil {
		return nil, err
	}

	digestTable := make(map[int]string, len(digests))
	for idx, d := range digests {
		digestTable[idx] = d
	}

	return digestTable, nil
}

// writeTrace writes a trace to the given writer.
func writeTrace(w io.Writer, paramKeyTable map[string]int, paramValTable map[string]int, digestTable map[string]int, id string, trace *types.GoldenTrace) error {
	// Write the id
	if err := writeBytesWithLength(w, []byte(id)); err != nil {
		return err
	}

	// Write parameters to bytes.
	if err := writeBytesWithLength(w, paramsToBytes(paramKeyTable, paramValTable, trace.Keys)); err != nil {
		return err
	}

	// Write values
	digests := make([]int, 0, len(trace.Digests))
	for _, d := range trace.Digests {
		digests = append(digests, digestTable[d])
	}

	_, err := w.Write(intsToBytes(digests))
	return err
}

// readTrace reads a trace from the given reader.
func readTrace(r io.Reader, keyTable map[int]string, valTable map[int]string, digestTable map[int]string, nCommits int) (string, *types.GoldenTrace, error) {
	// Read the id.
	byteId, err := readBytesWithLength(r)
	if err != nil {
		return "", nil, err
	}
	id := string(byteId)

	// Read the parameters.
	byteParams, err := readBytesWithLength(r)
	if err != nil {
		return "", nil, err
	}
	params, err := bytesToParams(keyTable, valTable, byteParams)
	if err != nil {
		return "", nil, err
	}

	// Read the values and convert them back from ints to strings.
	buffer := make([]byte, BYTES_PER_INT*nCommits)
	nBytesRead, err := r.Read(buffer)
	if err != nil {
		return "", nil, err
	}

	if nBytesRead != len(buffer) {
		return "", nil, fmt.Errorf("Read wrong number of bytes. Expected %d, got %d.", len(buffer), nBytesRead)
	}

	intDigests, err := bytesToInts(buffer)
	if err != nil {
		return "", nil, err
	}

	values := make([]string, 0, len(intDigests))
	for _, intDigest := range intDigests {
		values = append(values, digestTable[intDigest])
	}

	ret := &types.GoldenTrace{
		Keys:    params,
		Digests: values,
	}

	return id, ret, nil
}
