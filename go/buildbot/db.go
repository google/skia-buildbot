package buildbot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// build from the database.
func GetCommitsForBuild(master, builder string, buildNumber int) ([]string, error) {
	stmt, err := DB.Preparex(fmt.Sprintf("SELECT revision FROM %s WHERE master = ? AND builder = ? AND number = ?", TABLE_BUILD_REVISIONS))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve build revisions from database - failed to prepare query: %v", err)
	}
	defer stmt.Close()
	commits := []string{}
	if err := stmt.Select(&commits, master, builder, buildNumber); err != nil {
		return nil, fmt.Errorf("Unable to retrieve build revisions from database: %v", err)
	}
	return commits, nil
}

// GetBuildForCommit retrieves the build number of the build which first
// included the given commit.
func GetBuildForCommit(master, builder, commit string) (int, error) {
	stmt, err := DB.Preparex(fmt.Sprintf("SELECT number FROM %s WHERE master = ? AND builder = ? AND revision = ?", TABLE_BUILD_REVISIONS))
	if err != nil {
		return -1, fmt.Errorf("Unable to retrieve build number from database - failed to repare query: %v", err)
	}
	defer stmt.Close()
	n := -1
	if err := stmt.Get(&n, master, builder, commit); err != nil {
		if err == sql.ErrNoRows {
			// No build includes this commit.
			return -1, nil
		}
		return -1, fmt.Errorf("Unable to retrieve build number from database: %v", err)
	}
	return n, nil
}

// GetBuildFromDB retrieves the given build from the database as specified by
// the given master, builder, and build number.
func GetBuildFromDB(master, builder string, buildNumber int) (*Build, error) {
	// Get the build itself.
	stmt, err := DB.Preparex(fmt.Sprintf("SELECT * FROM %s WHERE master = ? AND builder = ? AND number = ?", TABLE_BUILDS))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve build from database - failed to prepare query: %v", err)
	}
	defer stmt.Close()
	build := Build{}
	if err = stmt.Get(&build, master, builder, buildNumber); err != nil {
		return nil, fmt.Errorf("Unable to retrieve build from database: %v", err)
	}

	// Build properties.
	var properties [][]interface{}
	if build.PropertiesStr != "" {
		if err = json.Unmarshal([]byte(build.PropertiesStr), &properties); err != nil {
			return nil, fmt.Errorf("Unable to parse build properties: %v", err)
		}
	}
	build.Properties = properties

	// Start and end times.
	build.Times = []float64{build.Started, build.Finished}

	// Get the steps.
	stepStmt, err := DB.Preparex(fmt.Sprintf("SELECT * FROM %s where master = ? AND builder = ? AND buildNumber = ?", TABLE_BUILD_STEPS))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve build steps from database - failed to prepare query: %v", err)
	}
	defer stepStmt.Close()
	steps := []*BuildStep{}
	if err := stepStmt.Select(&steps, master, builder, buildNumber); err != nil {
		return nil, fmt.Errorf("Unable to retrieve build steps from database: %v", err)
	}
	for _, s := range steps {
		s.Times = []float64{s.Started, s.Finished}
		s.ResultsRaw = []interface{}{float64(s.Results), []interface{}{}}
	}
	build.Steps = steps

	// Get the commits associated with this build.
	build.Commits, err = GetCommitsForBuild(master, builder, buildNumber)
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve commits for build: %v", err)
	}

	return &build, nil
}

// ReplaceIntoDB inserts or updates the Build in the database.
func (b Build) ReplaceIntoDB() error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = b.replaceIntoDB(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// replaceIntoDB inserts or updates the Build in the database.
func (b Build) replaceIntoDB() (rv error) {
	// Insert the build itself.
	tx, err := DB.Beginx()
	if err != nil {
		return fmt.Errorf("Unable to push build into database - Could not begin transaction: %v", err)
	}
	defer func() {
		if rv != nil {
			if err := tx.Rollback(); err != nil {
				err = fmt.Errorf("Failed to rollback the transaction! %v... Previous error: %v", err, rv)
			}
		} else {
			rv = tx.Commit()
			if rv != nil {
				tx.Rollback()
			} else {
			}
		}
	}()

	stmt, err := tx.Preparex(fmt.Sprintf("REPLACE INTO %s (master,builder,number,results,gotRevision,buildslave,started,finished,properties,branch) VALUES (?,?,?,?,?,?,?,?,?,?);", TABLE_BUILDS))
	if err != nil {
		return fmt.Errorf("Unable to push build into database - Could not prepare statement: %v", err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(b.MasterName, b.BuilderName, b.Number, b.Results, b.GotRevision, b.BuildSlave, b.Started, b.Finished, b.PropertiesStr, b.Branch)
	if err != nil {
		return fmt.Errorf("Failed to push build into database: %v", err)
	}

	// Build steps.

	// First, delete existing steps so that we don't have leftovers hanging
	// around from before.
	delStepsStmt, err := tx.Preparex(fmt.Sprintf("DELETE FROM %s WHERE master = ? AND builder = ? AND buildNumber = ?;", TABLE_BUILD_STEPS))
	if err != nil {
		return fmt.Errorf("Unable to delete build steps from database - Could not prepare statement: %v", err)
	}
	defer delStepsStmt.Close()
	_, err = delStepsStmt.Exec(b.MasterName, b.BuilderName, b.Number)
	if err != nil {
		return fmt.Errorf("Failed to delete build steps from database: %v", err)
	}
	// Actually insert the steps.
	insertStepStmt, err := tx.Preparex(fmt.Sprintf("REPLACE INTO %s (builder,master,buildNumber,name,results,number,started,finished) VALUES (?,?,?,?,?,?,?,?);", TABLE_BUILD_STEPS))
	if err != nil {
		return fmt.Errorf("Unable to push buildsteps into database - Could not prepare statement: %v", err)
	}
	defer insertStepStmt.Close()
	for _, s := range b.Steps {
		_, err = insertStepStmt.Exec(b.BuilderName, b.MasterName, b.Number, s.Name, s.Results, s.Number, s.Started, s.Finished)
		if err != nil {
			return fmt.Errorf("Failed to push build step into database: %v", err)
		}

	}

	// Commits.

	// First, delete existing revisions so that we don't have leftovers
	// hanging around from before.
	delCmtsStmt, err := tx.Preparex(fmt.Sprintf("DELETE FROM %s WHERE master = ? AND builder = ? AND number = ?;", TABLE_BUILD_REVISIONS))
	if err != nil {
		return fmt.Errorf("Unable to delete revisions from database - Could not prepare statement: %v", err)
	}
	defer delCmtsStmt.Close()
	_, err = delCmtsStmt.Exec(b.MasterName, b.BuilderName, b.Number)
	if err != nil {
		return fmt.Errorf("Failed to delete revisions from database: %v", err)
	}
	// Actually insert the commits.
	cmtStmt, err := tx.Preparex(fmt.Sprintf("REPLACE INTO %s (master,builder,number,revision) VALUES (?,?,?,?);", TABLE_BUILD_REVISIONS))
	if err != nil {
		return fmt.Errorf("Unable to push commits into database - Could not prepare statement: %v", err)
	}
	defer cmtStmt.Close()
	for _, c := range b.Commits {
		_, err = cmtStmt.Exec(b.MasterName, b.BuilderName, b.Number, c)
		if err != nil {
			return fmt.Errorf("Failed to push commit into database: %v", err)
		}
	}

	// The transaction is committed during the deferred function.
	return nil
}

// getLastProcessedBuilds returns a slice of INCOMPLETE builds where each build
// is the one with the greatest build number for its builder/master pair.
func getLastProcessedBuilds() ([]*Build, error) {
	stmt, err := DB.Preparex(fmt.Sprintf("SELECT master, builder, MAX(number) as number FROM %s GROUP BY builder, master;", TABLE_BUILDS))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve last-processed builds - Could not prepare statement: %v", err)
	}
	defer stmt.Close()
	builds := []*Build{}
	if err := stmt.Select(&builds); err != nil {
		return nil, fmt.Errorf("Unable to retrieve last-processed builds: %v", err)
	}
	return builds, nil
}

// getUnfinishedBuilds returns a slice of INCOMPLETE build objects representing
// the builds already entered into the database which were not finished at the
// time of their insertion. The only filled-in fields are MasterName,
// BuilderName, and Number.
func getUnfinishedBuilds() ([]*Build, error) {
	// Multiple steps. First, get the master/builder/number identifiers for
	// the unfinished builds, then run GetBuildFromDB for each one.
	stmt, err := DB.Preparex(fmt.Sprintf("SELECT master, builder, number FROM %s WHERE finished = 0;", TABLE_BUILDS))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve unfinished builds - Could not prepare statement: %v", err)
	}
	defer stmt.Close()
	b := []*Build{}
	if err = stmt.Select(&b); err != nil {
		return nil, fmt.Errorf("Unable to retrieve unfinished builds: %v", err)
	}
	return b, nil
}
