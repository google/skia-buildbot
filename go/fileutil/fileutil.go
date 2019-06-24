package fileutil

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// EnsureDirExists checks whether the given path to a directory exits and creates it
// if necessary. Returns the absolute path that corresponds to the input path
// and an error indicating a problem.
func EnsureDirExists(dirPath string) (string, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}

	return absPath, os.MkdirAll(absPath, 0700)
}

// EnsureDirPathExists checks whether the directories of the given file path
// exist and creates them if necessary. Returns an error if there was a problem
// creating the path.
func EnsureDirPathExists(dirPath string) error {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return err
	}

	dirs, _ := filepath.Split(absPath)
	if err := os.MkdirAll(dirs, 0700); err != nil {
		return err
	}

	return nil
}

// Must checks whether err in the provided pair (s, err) is nil. If so it
// returns s otherwise it cause the program to stop with the error message.
func Must(s string, err error) string {
	if err != nil {
		sklog.Fatal(err)
	}
	return s
}

// MustOpen opens the file of the given name, returning an *os.File on success, or causing the program to stop with the error message.
func MustOpen(name string) *os.File {
	f, err := os.Open(name)
	if err != nil {
		sklog.Fatal(err)
	}
	return f
}

// MustReaddir returns a slice of os.FileInfo for every file in the given dir.  This is equivalent to calling dir.Readdir(-1), except this call will stop the program on an error
func MustReaddir(dir *os.File) []os.FileInfo {
	fi, err := dir.Readdir(-1)
	if err != nil {
		sklog.Fatal(err)
	}
	return fi
}

// FileExists returns true if the given path exists and false otherwise.
// If there is an error it will return false and log the error message.
func FileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else if err != nil {
		sklog.Error(err)
		return false
	}
	return true
}

// TwoLevelRadixPath expands a path (defined by one or more path segments) by
// adding two additional directories based on the filename in the last segment.
// i.e.   TwoLevelRadixPath("/some", "path", "to", "abcdefgh.txt") will
// return "/some/path/to/ab/cd/abcdefgh.txt".
// If the filename does not have at least four characters before a period the
// return values is equivalent to filepath.Join(pathSegments...).
func TwoLevelRadixPath(pathSegments ...string) string {
	lastSeg := pathSegments[len(pathSegments)-1]
	dirName, fileName := filepath.Split(lastSeg)
	dotIdx := strings.Index(fileName, ".")
	if ((dotIdx < 4) && (dotIdx >= 0)) || ((dotIdx == -1) && (len(fileName) < 4)) {
		return filepath.Join(pathSegments...)
	}
	return filepath.Join(filepath.Join(pathSegments[:len(pathSegments)-1]...), filepath.Join(dirName, fileName[0:2], fileName[2:4], fileName))
}

// copyExecutable makes a byte-for-byte copy of the specified input file at the specified output
// location. It makes the permissions on the created file to be 755 (i.e. executable by all)
func CopyExecutable(fromPath, toPath string) error {
	data, err := ioutil.ReadFile(fromPath)
	if err != nil {
		return fmt.Errorf("Could not read file %s: %s", fromPath, err)
	}
	err = ioutil.WriteFile(toPath, data, 0755)
	if err != nil {
		return fmt.Errorf("Could not copy executable %s to %s: %s", fromPath, toPath, err)
	}
	return nil
}

// ReadAndSha1File opens a file, reads the contents, hashes them using the sha1
// hashing algorithm and returns the contents and hash.  If the file doesn't exist, the err will be
// nil and both contents and hash will be empty string.
func ReadAndSha1File(path string) (contents, hash string, err error) {
	if stat, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("Problem getting information for file %s: %s", path, err)
	} else if stat.IsDir() {
		return "", "", fmt.Errorf("Cannot open a directory: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("Problem opening file %s: %s", path, err)
	}
	defer util.Close(f)
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", "", fmt.Errorf("Problem reading file %s: %s", path, err)
	}

	return string(b), fmt.Sprintf("%x", sha1.Sum(b)), nil
}

// ReadLines opens the given path and reads the content as lines.
// It returns the lines without the trailing '\n' characters.
func ReadLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(file)

	result := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, err
}

// CountLines opens the given path and counts the number of lines in the file.
// Returns -1 with a non-nil error if an error is encountered.
func CountLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return -1, err
	}
	defer util.Close(file)

	numLines := 0
	// Using ReadLine instead of Scanner and ReadString because it can handle
	// lines longer than 65536 characters.
	r := bufio.NewReader(file)
	for {
		var isPrefix bool
		for {
			_, isPrefix, err = r.ReadLine()
			// If we've reached the end of the line, stop reading.
			if !isPrefix {
				break
			}
			// If we're just at the EOF, break
			if err != nil {
				break
			}
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return -1, err
		}
		numLines++
	}
	if err != io.EOF {
		return -1, err
	}

	return numLines, nil
}

// ReadAllFilesRecursive recursively reads all files in the given dir and
// returns a map of filename to contents.
func ReadAllFilesRecursive(dir string, excludeDirs []string) (map[string][]byte, error) {
	contents := map[string][]byte{}
	if err := filepath.Walk(dir, func(fp string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("Failed to walk filesystem: %s", err)
		}
		if info.IsDir() {
			base := filepath.Base(fp)
			if util.In(base, excludeDirs) {
				return filepath.SkipDir
			}
			return nil
		}
		b, err := ioutil.ReadFile(fp)
		if err != nil {
			return fmt.Errorf("Failed to read file: %s", err)
		}
		relpath := strings.TrimPrefix(strings.TrimPrefix(fp, dir), string(filepath.Separator))
		contents[relpath] = b
		return nil
	}); err != nil {
		return nil, fmt.Errorf("Failed to walk filesystem: %s", err)
	}
	return contents, nil
}
