package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/golden/go/sql"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func main() {
	dbName := flag.String("db_name", "", "The name of the db to put this data into")
	gitRepoPath := flag.String("git_repo_path", "", "The directory of the git repo to harvest data from. Be sure it's on the correct primary branch.")

	flag.Parse()
	if *dbName == "" || *gitRepoPath == "" {
		sklog.Fatalf("Must provide --db_name and --git_repo_path")
	}

	ctx := context.Background()

	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		`--execute=CREATE DATABASE IF NOT EXISTS `+*dbName).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Error creating database %s: %s\n %s", *dbName, err, out)
	}

	dbCmd := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		"--database="+*dbName,
		"--execute="+sql.CockroachDBSchema)
	out, err = dbCmd.CombinedOutput()
	if err != nil {
		sklog.Fatalf("Could not create schema: %s %s", err, out)
	}

	sklog.Infof("Getting revision list from %s", *gitRepoPath)
	gitCmd := exec.CommandContext(ctx, "git", "rev-list", "HEAD", `--pretty=%aN <%aE>%n%s%n%ct`, "--reverse")
	gitCmd.Dir = *gitRepoPath

	stdout, err := gitCmd.StdoutPipe()
	if err != nil {
		sklog.Fatalf("Getting stdout: %s", err)
	}
	if err := gitCmd.Start(); err != nil {
		sklog.Fatalf("Starting command; is git on the path?: %s", err)
	}

	var commitsToStore []sqlCommit
	commitNumber := 0
	err = parseGitRevLogStream(stdout, func(p sqlCommit) error {
		p.CommitID = commitNumber
		commitsToStore = append(commitsToStore, p)
		commitNumber++
		return nil
	})
	if err != nil {
		// Once we've successfully called gitCmd.Start() we must always call
		// gitCmd.Wait() to close stdout.
		_ = gitCmd.Wait()
		sklog.Fatalf("getting the revision list: %s", skerr.Wrap(err))
	}

	if err := gitCmd.Wait(); err != nil {
		exerr := err.(*exec.ExitError)
		sklog.Fatalf("Failed to go through repo history: %s %s", err, exerr.Stderr)
	}

	sklog.Infof("Found %d revisions: %#v", len(commitsToStore), commitsToStore[len(commitsToStore)-5:])

	db, err := pgxpool.Connect(ctx, "postgresql://root@localhost:26257/"+*dbName+"?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	defer db.Close()

	const chunkSize = 1000
	err = util.ChunkIter(len(commitsToStore), chunkSize, func(startIdx int, endIdx int) error {
		sklog.Debugf("Storing Batch [%d:%d]", startIdx, endIdx)
		batch := commitsToStore[startIdx:endIdx]

		upsertStatement := "UPSERT INTO Commits (commit_id, git_hash, commit_time, author, subject) VALUES "

		var arguments []interface{}
		argumentIdx := 1
		for i, c := range batch {
			if i != 0 {
				upsertStatement += ","
			}
			upsertStatement += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)",
				argumentIdx, argumentIdx+1, argumentIdx+2, argumentIdx+3, argumentIdx+4)
			argumentIdx += 5

			arguments = append(arguments, c.CommitID)
			arguments = append(arguments, c.GitHash)
			arguments = append(arguments, c.Timestamp)
			arguments = append(arguments, c.Author)
			arguments = append(arguments, c.Subject)
		}
		_, err := db.Exec(ctx, upsertStatement, arguments...)
		return err
	})
	if err != nil {
		sklog.Fatalf("Could not store to SQL %s", err)
	}
	sklog.Infof("Done")
}

// sqlCommit represents a single commit stored in the database.
type sqlCommit struct {
	CommitID  int
	GitHash   string
	Timestamp time.Time
	Author    string
	Subject   string
}

// Forked from Perf's version

type parseGitRevLogStreamProcessSingleCommit func(commit sqlCommit) error

// parseGitRevLogStream parses the input stream for input of the form:
//
//     commit 6079a7810530025d9877916895dd14eb8bb454c0
//     Joe Gregorio <joe@bitworking.org>
//     Change #9
//     1584837783
//     commit 977e0ef44bec17659faf8c5d4025c5a068354817
//     Joe Gregorio <joe@bitworking.org>
//     Change #8
//     1584837783
//
// And calls the parseGitRevLogStreamProcessSingleCommit function with each
// entry it finds. The passed in sqlCommit has all valid fields except
// CommitNumber, which is set to types.BadCommitNumber.
func parseGitRevLogStream(r io.ReadCloser, f parseGitRevLogStreamProcessSingleCommit) error {
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "commit ") {
			return skerr.Fmt("Invalid format, expected commit at line %d: %q", lineNumber, line)
		}
		lineNumber++
		gitHash := strings.Split(line, " ")[1]

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting an author line: %d", lineNumber)
		}
		lineNumber++
		author := scanner.Text()

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting a subject line: %d", lineNumber)
		}
		lineNumber++
		subject := scanner.Text()

		if !scanner.Scan() {
			return skerr.Fmt("Ran out of input, expecting a timestamp line: %d", lineNumber)
		}
		lineNumber++
		timestampString := scanner.Text()
		ts, err := strconv.ParseInt(timestampString, 10, 64)
		if err != nil {
			return skerr.Fmt("Failed to parse timestamp %q at line %d", timestampString, lineNumber)
		}
		if err := f(sqlCommit{
			GitHash:   gitHash,
			Timestamp: time.Unix(ts, 0),
			Author:    author,
			Subject:   subject,
		}); err != nil {
			return skerr.Wrap(err)
		}
	}
	return skerr.Wrap(scanner.Err())
}
