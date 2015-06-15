package buildbot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
)

// buildFromDB is a convenience struct which handles nullable database fields.
type buildFromDB struct {
	Id          int64           `db:"id"`
	Builder     string          `db:"builder"`
	Master      string          `db:"master"`
	Number      int             `db:"number"`
	GotRevision sql.NullString  `db:"gotRevision"`
	Branch      string          `db:"branch"`
	Results     sql.NullInt64   `db:"results"`
	BuildSlave  string          `db:"buildslave"`
	Started     sql.NullFloat64 `db:"started"`
	Finished    sql.NullFloat64 `db:"finished"`
	Properties  sql.NullString  `db:"properties"`
	Repository  string          `db:"repository"`
}

func (b buildFromDB) toBuild() *Build {
	return &Build{
		Id:            int(b.Id),
		Builder:       b.Builder,
		Master:        b.Master,
		Number:        b.Number,
		GotRevision:   b.GotRevision.String,
		Branch:        b.Branch,
		Results:       int(b.Results.Int64),
		BuildSlave:    b.BuildSlave,
		Started:       b.Started.Float64,
		Finished:      b.Finished.Float64,
		PropertiesStr: b.Properties.String,
		Repository:    b.Repository,
	}
}

// buildStepFromDB is a convenience struct which handles nullable database fields.
type buildStepFromDB struct {
	Id       int64           `db:"id"`
	BuildID  int64           `db:"buildId"`
	Name     string          `db:"name"`
	Number   int             `db:"number"`
	Results  sql.NullInt64   `db:"results"`
	Started  sql.NullFloat64 `db:"started"`
	Finished sql.NullFloat64 `db:"finished"`
}

func (s buildStepFromDB) toBuildStep() *BuildStep {
	return &BuildStep{
		Id:       int(s.Id),
		BuildID:  int(s.BuildID),
		Name:     s.Name,
		Number:   s.Number,
		Results:  int(s.Results.Int64),
		Started:  s.Started.Float64,
		Finished: s.Finished.Float64,
	}
}

// commitFromDB is a convenience struct for storing commits for a build.
type commitFromDB struct {
	Id       int    `db:"id"`
	BuildID  int    `db:"buildId"`
	Revision string `db:"revision"`
}

// GetBuildForCommit retrieves the build number of the build which first
// included the given commit.
func GetBuildForCommit(builder, master, commit string) (int, error) {
	n := -1
	if err := DB.Get(&n, fmt.Sprintf("SELECT number FROM %s WHERE id IN (SELECT buildId FROM %s WHERE revision = ?) AND builder = ? AND master = ?;", TABLE_BUILDS, TABLE_BUILD_REVISIONS), commit, builder, master); err != nil {
		if err == sql.ErrNoRows {
			// No build includes this commit.
			return -1, nil
		}
		return -1, fmt.Errorf("Unable to retrieve build number from database: %v", err)
	}
	return n, nil
}

// GetBuildIDsForCommit retrieves IDs for all builds which first included the
// given commit.
func GetBuildIDsForCommit(commit string) ([]*BuildID, error) {
	buildIDs := []*BuildID{}
	if err := DB.Select(&buildIDs, fmt.Sprintf("SELECT buildId FROM %s WHERE revision = ?;", TABLE_BUILD_REVISIONS), commit); err != nil {
		if err == sql.ErrNoRows {
			// No builds include this commit.
			return []*BuildID{}, nil
		}
		return nil, fmt.Errorf("Unable to retrieve builds for commit: %v", err)
	}
	return buildIDs, nil
}

// GetBuildIDsForCommits retrieves IDs for all builds which first included each
// of the given commits.
func GetBuildIDsForCommits(commits []string) (map[string][]int, error) {
	res := []struct {
		Revision string `db:"revision"`
		BuildId  int    `db:"buildId"`
	}{}
	commitsInterface := make([]interface{}, 0, len(commits))
	for _, c := range commits {
		commitsInterface = append(commitsInterface, c)
	}
	tmpl := util.RepeatJoin("?", ",", len(commitsInterface))
	if err := DB.Select(&res, fmt.Sprintf("SELECT revision, buildId FROM %s WHERE revision IN (%s);", TABLE_BUILD_REVISIONS, tmpl), commitsInterface...); err != nil {
		if err == sql.ErrNoRows {
			// No builds include these commits.
			return map[string][]int{}, nil
		}
		return nil, fmt.Errorf("Unable to retrieve builds for commits: %v", err)
	}
	rv := map[string][]int{}
	for _, r := range res {
		if v, ok := rv[r.Revision]; !ok || v == nil {
			rv[r.Revision] = []int{}
		}
		rv[r.Revision] = append(rv[r.Revision], r.BuildId)
	}
	return rv, nil
}

// GetBuildsForCommits retrieves all builds which first included each of the
// given commits.
func GetBuildsForCommits(commits []string, ignore map[int]bool) (map[string][]*Build, error) {
	// Get the set of build IDs by commit hash.
	idsByCommit, err := GetBuildIDsForCommits(commits)
	if err != nil {
		return nil, err
	}
	// Shortcut: If we got nothing back, just return.
	if len(idsByCommit) == 0 {
		return map[string][]*Build{}, nil
	}
	// Make a de-duplicated list of build IDs to retrieve.
	idMap := map[int]bool{}
	for _, idList := range idsByCommit {
		for _, id := range idList {
			if !ignore[id] {
				idMap[id] = true
			}
		}
	}
	ids := make([]int, 0, len(idMap))
	for id, _ := range idMap {
		ids = append(ids, id)
	}
	// Retrieve the builds.
	builds, err := GetBuildsFromDB(ids)
	if err != nil {
		return nil, err
	}
	// Organize the builds by commit.
	buildsByCommit := map[string][]*Build{}
	for commit, buildIds := range idsByCommit {
		buildsByCommit[commit] = make([]*Build, 0, len(buildIds))
		for _, id := range buildIds {
			if builds[id] == nil {
				continue
			}
			buildsByCommit[commit] = append(buildsByCommit[commit], builds[id])
		}
	}
	return buildsByCommit, nil
}

// GetBuildIDFromDB retrieves the ID for the given build from the database
// as specified by the given master, builder, and build number.
func GetBuildIDFromDB(builder, master string, buildNumber int) (int, error) {
	var id int
	stmt := fmt.Sprintf("SELECT id FROM %s WHERE builder = ? AND master = ? AND number = ?", TABLE_BUILDS)
	if err := DB.Get(&id, stmt, builder, master, buildNumber); err != nil {
		return -1, fmt.Errorf("Unable to retrieve build ID from database: %v", err)
	}
	return id, nil
}

// GetBuildFromDB retrieves the given build from the database as specified by
// the given master, builder, and build number.
func GetBuildFromDB(builder, master string, buildNumber int) (*Build, error) {
	// Get the build itself.
	b := buildFromDB{}
	if err := DB.Get(&b, fmt.Sprintf("SELECT * FROM %s WHERE builder = ? AND master = ? AND number = ?", TABLE_BUILDS), builder, master, buildNumber); err != nil {
		return nil, fmt.Errorf("Unable to retrieve build from database: %v", err)
	}
	build := b.toBuild()

	// Build properties.
	var properties [][]interface{}
	if build.PropertiesStr != "" {
		if err := json.Unmarshal([]byte(build.PropertiesStr), &properties); err != nil {
			return nil, fmt.Errorf("Unable to parse build properties: %v", err)
		}
	}
	build.Properties = properties

	// Start and end times.
	build.Times = []float64{build.Started, build.Finished}

	var wg sync.WaitGroup

	// Get the steps.
	steps := []*BuildStep{}
	var stepsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		stepsFromDB := []*buildStepFromDB{}
		if err := DB.Select(&stepsFromDB, fmt.Sprintf("SELECT * FROM %s WHERE buildId = ?", TABLE_BUILD_STEPS), build.Id); err != nil {
			stepsErr = fmt.Errorf("Unable to retrieve build steps from database: %v", err)
			return
		}
		steps = make([]*BuildStep, len(stepsFromDB))
		for i, s := range stepsFromDB {
			step := s.toBuildStep()
			step.Times = []float64{step.Started, step.Finished}
			step.ResultsRaw = []interface{}{float64(step.Results), []interface{}{}}
			steps[i] = step
		}
	}()

	// Get the commits associated with this build.
	commits := []string{}
	var commitsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := DB.Select(&commits, fmt.Sprintf("SELECT revision FROM %s WHERE buildId = ?;", TABLE_BUILD_REVISIONS), build.Id); err != nil {
			commitsErr = fmt.Errorf("Unable to retrieve build revisions from database: %v", err)
			return
		}
	}()

	// Get the comments on this build.
	comments := []*BuildComment{}
	var commentsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := DB.Select(&comments, fmt.Sprintf("SELECT * FROM %s WHERE buildId = ?;", TABLE_BUILD_COMMENTS), build.Id); err != nil {
			commitsErr = fmt.Errorf("Unable to retrieve build comments from database: %v", err)
			return
		}
	}()

	wg.Wait()

	// Return error if any, or the result.
	if stepsErr != nil {
		return nil, stepsErr
	}
	if commitsErr != nil {
		return nil, commitsErr
	}
	if commentsErr != nil {
		return nil, commentsErr
	}

	build.Steps = steps
	build.Commits = commits
	build.Comments = comments
	return build, nil
}

// GetBuildsFromDB retrieves the given builds from the database.
func GetBuildsFromDB(ids []int) (map[int]*Build, error) {
	if len(ids) == 0 {
		return map[int]*Build{}, nil
	}
	interfaceIds := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		interfaceIds = append(interfaceIds, id)
	}
	inputTmpl := util.RepeatJoin("?", ",", len(interfaceIds))

	var wg sync.WaitGroup

	// Get builds
	var buildsById map[int]*Build
	var buildsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		b := []*buildFromDB{}
		if err := DB.Select(&b, fmt.Sprintf("SELECT * FROM %s WHERE id IN (%s);", TABLE_BUILDS, inputTmpl), interfaceIds...); err != nil {
			buildsErr = fmt.Errorf("Could not retrieve builds: %v", err)
			return
		}
		buildsById = map[int]*Build{}
		for _, buildFromDB := range b {
			build := buildFromDB.toBuild()
			// Build properties.
			var properties [][]interface{}
			if build.PropertiesStr != "" {
				if err := json.Unmarshal([]byte(build.PropertiesStr), &properties); err != nil {
					buildsErr = fmt.Errorf("Unable to parse build properties: %v", err)
				}
			}
			build.Properties = properties

			// Start and end times.
			build.Times = []float64{build.Started, build.Finished}
			buildsById[build.Id] = build
		}
	}()

	// Build steps.
	stepsFromDB := []*buildStepFromDB{}
	var stepsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := DB.Select(&stepsFromDB, fmt.Sprintf("SELECT * FROM %s WHERE buildId IN (%s);", TABLE_BUILD_STEPS, inputTmpl), interfaceIds...); err != nil {
			stepsErr = fmt.Errorf("Could not retrieve build steps from database: %v", err)
			return
		}
	}()

	// Commits for each build.
	commitsFromDB := []*commitFromDB{}
	var commitsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := DB.Select(&commitsFromDB, fmt.Sprintf("SELECT * FROM %s WHERE buildId IN (%s);", TABLE_BUILD_REVISIONS, inputTmpl), interfaceIds...); err != nil {
			commitsErr = fmt.Errorf("Could not retrieve revisions from database: %v", err)
			return
		}
	}()

	// Comments on each build.
	commentsFromDB := []*BuildComment{}
	var commentsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := DB.Select(&commentsFromDB, fmt.Sprintf("SELECT * FROM %s WHERE buildId IN (%s);", TABLE_BUILD_COMMENTS, inputTmpl), interfaceIds...); err != nil {
			commentsErr = fmt.Errorf("Could not retrieve comments from database: %v", err)
			return
		}
	}()

	wg.Wait()

	// Return error if applicable.
	if buildsErr != nil {
		return nil, buildsErr
	}
	if stepsErr != nil {
		return nil, stepsErr
	}
	if commitsErr != nil {
		return nil, commitsErr
	}
	if commentsErr != nil {
		return nil, commentsErr
	}

	// Associate steps with builds.
	for _, stepFromDB := range stepsFromDB {
		s := stepFromDB.toBuildStep()
		s.Times = []float64{s.Started, s.Finished}
		s.ResultsRaw = []interface{}{float64(s.Results), []interface{}{}}
		build, ok := buildsById[s.BuildID]
		if !ok {
			return nil, fmt.Errorf("Failed to retrieve builds; got a build step with no associated build.")
		}
		if build.Steps == nil {
			build.Steps = []*BuildStep{}
		}
		build.Steps = append(build.Steps, s)
	}

	// Associate commits with builds.
	for _, c := range commitsFromDB {
		build, ok := buildsById[c.BuildID]
		if !ok {
			return nil, fmt.Errorf("Failed to retrieve builds; got a commit with no associated build.")
		}
		if build.Commits == nil {
			build.Commits = []string{}
		}
		build.Commits = append(build.Commits, c.Revision)
	}

	// Associate comments with builds.
	for _, c := range commentsFromDB {
		build, ok := buildsById[c.BuildId]
		if !ok {
			return nil, fmt.Errorf("Failed to retrieve builds; got a comment with no associated build.")
		}
		if build.Comments == nil {
			build.Comments = []*BuildComment{}
		}
		build.Comments = append(build.Comments, c)
	}
	return buildsById, nil
}

// ReplaceIntoDB inserts or updates the Build in the database.
func (b *Build) ReplaceIntoDB() error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = b.replaceIntoDB(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// replaceStepsIntoDB inserts, updates, or deletes the given BuildSteps into
// the database. Returns a map whose keys are pointers to any newly-inserted
// BuildSteps and whose values are the IDs which should be assigned to those
// steps. This allows us to postpone assigning any IDs until the transaction
// has completed successfully.
func replaceStepsIntoDB(tx *sqlx.Tx, steps []*BuildStep, newBuildID int) (map[*BuildStep]int, error) {
	// First, determine which steps need to be inserted, updated, and deleted.
	stepsFromDB := []*buildStepFromDB{}
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE buildId = ?;", TABLE_BUILD_STEPS)
	if err := tx.Select(&stepsFromDB, stmt, newBuildID); err != nil {
		return nil, fmt.Errorf("Could not retrieve build steps from database: %v", err)
	}
	oldSteps := make(map[int]bool, len(stepsFromDB))
	for _, s := range stepsFromDB {
		oldSteps[int(s.Id)] = true
	}
	newSteps := make(map[int]*BuildStep, len(steps))
	for _, s := range steps {
		newSteps[s.Id] = s
	}
	update := make([]*BuildStep, 0, len(steps))
	insert := make([]*BuildStep, 0, len(steps))
	remove := make([]int, 0, len(steps))
	for _, s := range steps {
		if _, ok := oldSteps[s.Id]; ok {
			update = append(update, s)
			delete(oldSteps, s.Id)
		} else {
			insert = append(insert, s)
		}
	}
	for id, _ := range oldSteps {
		remove = append(remove, id)
	}

	rv := map[*BuildStep]int{}

	// Delete any no-longer-existing steps.
	if len(remove) > 0 {
		idTmpl := util.RepeatJoin("?", ",", len(remove))
		removeIds := make([]interface{}, 0, len(remove))
		for _, id := range remove {
			removeIds = append(removeIds, id)
		}
		stmt := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", TABLE_BUILD_STEPS, idTmpl)
		if _, err := tx.Exec(stmt, removeIds...); err != nil {
			return nil, fmt.Errorf("Could not remove old build steps.")
		}
	}

	// Insert any new steps.
	for _, s := range insert {
		stmt := fmt.Sprintf("INSERT INTO %s (buildId,name,results,number,started,finished) VALUES (?,?,?,?,?,?);", TABLE_BUILD_STEPS)
		res, err := tx.Exec(stmt, newBuildID, s.Name, s.Results, s.Number, s.Started, s.Finished)
		if err != nil {
			return nil, fmt.Errorf("Unable to push buildsteps into database: %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("Unable to get ID for inserted buildstep: %v", err)
		}
		rv[s] = int(id)
	}

	// Update any already-existing steps.
	for _, s := range update {
		stmt := fmt.Sprintf("UPDATE %s SET buildId=?, name=?, results=?, number=?, started=?, finished=? WHERE id = ?", TABLE_BUILD_STEPS)
		if _, err := tx.Exec(stmt, s.BuildID, s.Name, s.Results, s.Number, s.Started, s.Finished, s.Id); err != nil {
			return nil, fmt.Errorf("Failed to update build steps: %v", err)
		}
	}

	return rv, nil
}

// replaceCommitsIntoDB inserts, updates, or deletes the commits for the
// build into the database.
func replaceCommitsIntoDB(tx *sqlx.Tx, commits []string, buildID int) error {
	// First, determine which commits need to be inserted, updated, and deleted.
	commitsFromDB := []*commitFromDB{}
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE buildId = ?;", TABLE_BUILD_REVISIONS)
	if err := DB.Select(&commitsFromDB, stmt, buildID); err != nil {
		return fmt.Errorf("Could not retrieve commits from database: %v", err)
	}
	oldCommits := make(map[string]*commitFromDB, len(commitsFromDB))
	for _, c := range commitsFromDB {
		oldCommits[c.Revision] = c
	}
	newCommits := make(map[string]bool, len(commits))
	for _, c := range commits {
		newCommits[c] = true
	}
	update := make([]*commitFromDB, 0, len(commits))
	insert := make([]*commitFromDB, 0, len(commits))
	remove := make([]int, 0, len(commits))
	for _, c := range commits {
		if old, ok := oldCommits[c]; ok {
			update = append(update, old)
			delete(oldCommits, old.Revision)
		} else {
			insert = append(insert, &commitFromDB{
				BuildID:  buildID,
				Revision: c,
			})
		}
	}
	for _, c := range oldCommits {
		remove = append(remove, c.Id)
	}

	// Delete any no-longer-existing commits.
	if len(remove) > 0 {
		idTmpl := util.RepeatJoin("?", ",", len(remove))
		removeIds := make([]interface{}, 0, len(remove))
		for _, id := range remove {
			removeIds = append(removeIds, id)
		}
		stmt := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", TABLE_BUILD_REVISIONS, idTmpl)
		if _, err := tx.Exec(stmt, removeIds...); err != nil {
			return fmt.Errorf("Could not remove old build revisions.")
		}
	}

	// Insert any new commits.
	for _, c := range insert {
		c.BuildID = buildID
		stmt := fmt.Sprintf("INSERT INTO %s (buildId,revision) VALUES (?,?);", TABLE_BUILD_REVISIONS)
		if _, err := tx.Exec(stmt, c.BuildID, c.Revision); err != nil {
			return fmt.Errorf("Unable to push revisions into database: %v", err)
		}
	}

	// Update any already-existing commits.
	for _, c := range update {
		stmt := fmt.Sprintf("UPDATE %s SET buildId=?, revision=? WHERE id = ?", TABLE_BUILD_REVISIONS)
		if _, err := tx.Exec(stmt, c.BuildID, c.Revision, c.Id); err != nil {
			return fmt.Errorf("Failed to update build revisions: %v", err)
		}
	}

	return nil
}

// replaceCommentsIntoDB inserts, updates, or deletes the given BuildComments
// into the database. Returns a map whose keys are pointers to any
// newly-inserted BuildComments and whose values are the IDs which should be
// assigned to those comments. This allows us to postpone assigning any IDs
// until the transaction has completed successfully.
func replaceCommentsIntoDB(tx *sqlx.Tx, comments []*BuildComment, newBuildID int) (map[*BuildComment]int, error) {
	// First, determine which comments need to be inserted, updated, and deleted.
	commentsFromDB := []*BuildComment{}
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE buildId = ?;", TABLE_BUILD_COMMENTS)
	if err := tx.Select(&commentsFromDB, stmt, newBuildID); err != nil {
		return nil, fmt.Errorf("Could not retrieve build comments from database: %v", err)
	}
	oldComments := make(map[int]bool, len(commentsFromDB))
	for _, c := range commentsFromDB {
		oldComments[int(c.Id)] = true
	}
	newComments := make(map[int]*BuildComment, len(comments))
	for _, c := range comments {
		newComments[c.Id] = c
	}
	update := make([]*BuildComment, 0, len(comments))
	insert := make([]*BuildComment, 0, len(comments))
	remove := make([]int, 0, len(comments))
	for _, c := range comments {
		if _, ok := oldComments[c.Id]; ok {
			update = append(update, c)
			delete(oldComments, c.Id)
		} else {
			insert = append(insert, c)
		}
	}
	for id, _ := range oldComments {
		remove = append(remove, id)
	}

	rv := map[*BuildComment]int{}

	// Delete any no-longer-existing comments.
	if len(remove) > 0 {
		idTmpl := util.RepeatJoin("?", ",", len(remove))
		removeIds := make([]interface{}, 0, len(remove))
		for _, id := range remove {
			removeIds = append(removeIds, id)
		}
		stmt := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", TABLE_BUILD_COMMENTS, idTmpl)
		if _, err := tx.Exec(stmt, removeIds...); err != nil {
			return nil, fmt.Errorf("Could not remove old build comments.")
		}
	}

	// Insert any new comments.
	for _, c := range insert {
		stmt := fmt.Sprintf("INSERT INTO %s (buildId,user,timestamp,message) VALUES (?,?,?,?);", TABLE_BUILD_COMMENTS)
		res, err := tx.Exec(stmt, newBuildID, c.User, c.Timestamp, c.Message)
		if err != nil {
			return nil, fmt.Errorf("Unable to push build comments into database: %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("Unable to get ID for inserted build comment: %v", err)
		}
		rv[c] = int(id)
	}

	// Update any already-existing comments.
	for _, c := range update {
		stmt := fmt.Sprintf("UPDATE %s SET buildId=?, user=?, timestamp=?, message=? WHERE id = ?", TABLE_BUILD_COMMENTS)
		if _, err := tx.Exec(stmt, c.BuildId, c.User, c.Timestamp, c.Message, c.Id); err != nil {
			return nil, fmt.Errorf("Failed to update build comments: %v", err)
		}
	}

	return rv, nil
}

// replaceIntoDB inserts or updates the Build in the database.
func (b *Build) replaceIntoDB() (rv error) {
	// Insert the build itself.
	tx, err := DB.Beginx()
	if err != nil {
		return fmt.Errorf("Unable to push build into database - Could not begin transaction: %v", err)
	}

	buildID := b.Id
	var stepIDMap map[*BuildStep]int
	var commentIDMap map[*BuildComment]int

	// Defer committing/rolling back the transaction. If successful,
	// update the Build with new IDs.
	defer func() {
		rv = database.CommitOrRollback(tx, rv)
		if rv == nil {
			// Update the build with any new IDs.
			b.Id = buildID
			for _, s := range b.Steps {
				s.BuildID = b.Id
			}
			for s, id := range stepIDMap {
				s.Id = id
			}
			for c, id := range commentIDMap {
				c.Id = id
			}
		}
	}()

	if buildID == 0 {
		stmt := fmt.Sprintf("INSERT INTO %s (builder,master,number,results,gotRevision,buildslave,started,finished,properties,branch,repository) VALUES (?,?,?,?,?,?,?,?,?,?,?);", TABLE_BUILDS)
		res, err := tx.Exec(stmt, b.Builder, b.Master, b.Number, b.Results, b.GotRevision, b.BuildSlave, b.Started, b.Finished, b.PropertiesStr, b.Branch, b.Repository)
		if err != nil {
			return fmt.Errorf("Failed to push build into database: %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("Failed to push build into database; LastInsertId failed: %v", err)
		}
		buildID = int(id)
	} else {
		stmt := fmt.Sprintf("UPDATE %s set builder=?, master=?, number=?, results=?, gotRevision=?, buildslave=?, started=?, finished=?, properties=?, branch=?, repository=? WHERE id=?;", TABLE_BUILDS)
		if _, err := tx.Exec(stmt, b.Builder, b.Master, b.Number, b.Results, b.GotRevision, b.BuildSlave, b.Started, b.Finished, b.PropertiesStr, b.Branch, b.Repository, buildID); err != nil {
			return fmt.Errorf("Failed to push build into database: %v", err)
		}
	}

	// Build steps.
	stepIDMap, err = replaceStepsIntoDB(tx, b.Steps, buildID)
	if err != nil {
		return err
	}

	// Commits.
	if err := replaceCommitsIntoDB(tx, b.Commits, buildID); err != nil {
		return err
	}

	// Comments.
	commentIDMap, err = replaceCommentsIntoDB(tx, b.Comments, buildID)
	if err != nil {
		return err
	}

	// The transaction is committed during the deferred function.
	return nil
}

// getLastProcessedBuilds returns a slice of BuildIDs where each build
// is the one with the greatest build number for its builder/master pair.
func getLastProcessedBuilds() ([]*BuildID, error) {
	buildIds := []*BuildID{}
	if err := DB.Select(&buildIds, fmt.Sprintf("SELECT master, builder, MAX(number) as number FROM %s GROUP BY builder, master;", TABLE_BUILDS)); err != nil {
		return nil, fmt.Errorf("Unable to retrieve last-processed builds: %v", err)
	}
	return buildIds, nil
}

// getUnfinishedBuilds returns a slice of BuildIDs for the builds already
// entered into the database which were not finished at the time of their
// insertion.
func getUnfinishedBuilds() ([]*BuildID, error) {
	b := []*BuildID{}
	if err := DB.Select(&b, fmt.Sprintf("SELECT builder,master,number FROM %s WHERE finished = 0;", TABLE_BUILDS)); err != nil {
		return nil, fmt.Errorf("Unable to retrieve unfinished builds: %v", err)
	}
	return b, nil
}

// NumIngestedBuilds returns the total number of builds which have been
// ingested into the database.
func NumIngestedBuilds() (int, error) {
	i := 0
	if err := DB.Get(&i, fmt.Sprintf("SELECT COUNT(*) FROM %s;", TABLE_BUILDS)); err != nil {
		return 0, fmt.Errorf("Unable to find the number of ingested builds: %s", err)
	}
	return i, nil
}

// GetBuilderComments returns the comments for the given builder.
func GetBuilderComments(builder string) ([]*BuilderComment, error) {
	c := []*BuilderComment{}
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE builder = ?;", TABLE_BUILDER_COMMENTS)
	if err := DB.Get(&c, stmt, builder); err != nil {
		if err == sql.ErrNoRows {
			// No comments for this builder, just return an empty slice with no error.
			return c, nil
		}
		return nil, fmt.Errorf("Unable to retrieve builder comments: %v", err)
	}
	return c, nil
}

// GetBuildersComments returns the comments for each of the given builders.
func GetBuildersComments(builders []string) (map[string][]*BuilderComment, error) {
	if len(builders) == 0 {
		return map[string][]*BuilderComment{}, nil
	}
	buildersInterface := make([]interface{}, 0, len(builders))
	for _, b := range builders {
		buildersInterface = append(buildersInterface, b)
	}
	tmpl := util.RepeatJoin("?", ",", len(buildersInterface))
	c := []*BuilderComment{}
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE builder IN (%s);", TABLE_BUILDER_COMMENTS, tmpl)
	if err := DB.Select(&c, stmt, buildersInterface...); err != nil {
		if err == sql.ErrNoRows {
			// None of these builders have comments.
			return map[string][]*BuilderComment{}, nil
		}
		return nil, fmt.Errorf("Unable to retrieve comment for builders: %v", err)
	}
	rv := map[string][]*BuilderComment{}
	for _, comment := range c {
		if _, ok := rv[comment.Builder]; !ok {
			rv[comment.Builder] = []*BuilderComment{}
		}
		rv[comment.Builder] = append(rv[comment.Builder], comment)
	}
	return rv, nil
}

// InsertIntoDB inserts the BuilderComment into the database.
func (c *BuilderComment) InsertIntoDB() error {
	if c.Id != 0 {
		return fmt.Errorf("BuilderComment has non-zero ID %d; has it already been inserted?", c.Id)
	}
	stmt := fmt.Sprintf("INSERT INTO %s (builder,user,timestamp,flaky,ignoreFailure,message) VALUES (?,?,?,?,?,?);", TABLE_BUILDER_COMMENTS)
	res, err := DB.Exec(stmt, c.Builder, c.User, c.Timestamp, c.Flaky, c.IgnoreFailure, c.Message)
	if err != nil {
		return fmt.Errorf("Unable to insert builder status: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("Unable to get last insert ID: %v", err)
	}
	c.Id = int(id)
	return nil
}

// DeleteBuilderComment deletes the BuilderComment from the database.
func DeleteBuilderComment(id int) error {
	stmt := fmt.Sprintf("DELETE FROM %s WHERE id = ?", TABLE_BUILDER_COMMENTS)
	if _, err := DB.Exec(stmt, id); err != nil {
		return fmt.Errorf("Failed to delete builder comment: %v", err)
	}
	return nil
}

// GetCommitComments returns the comments on the given commit.
func GetCommitComments(commit string) ([]*CommitComment, error) {
	c := []*CommitComment{}
	if err := DB.Select(&c, fmt.Sprintf("SELECT * FROM %s WHERE commit = ?", TABLE_COMMIT_COMMENTS), commit); err != nil {
		return nil, fmt.Errorf("Unable to retrieve commit comments: %v", err)
	}
	return c, nil
}

// GetCommitsComments returns the comments on each of the given commits.
func GetCommitsComments(commits []string) (map[string][]*CommitComment, error) {
	if len(commits) == 0 {
		return map[string][]*CommitComment{}, nil
	}
	commitsInterface := make([]interface{}, 0, len(commits))
	for _, c := range commits {
		commitsInterface = append(commitsInterface, c)
	}
	tmpl := util.RepeatJoin("?", ",", len(commitsInterface))
	c := []*CommitComment{}
	if err := DB.Select(&c, fmt.Sprintf("SELECT * FROM %s WHERE commit IN (%s);", TABLE_COMMIT_COMMENTS, tmpl), commitsInterface...); err != nil {
		if err == sql.ErrNoRows {
			// None of these commits have comments.
			return map[string][]*CommitComment{}, nil
		}
		return nil, fmt.Errorf("Unable to retrieve comments for commits: %v", err)
	}
	rv := map[string][]*CommitComment{}
	for _, comment := range c {
		if _, ok := rv[comment.Commit]; ok {
			rv[comment.Commit] = append(rv[comment.Commit], comment)
		} else {
			rv[comment.Commit] = []*CommitComment{comment}
		}
	}
	return rv, nil
}

// InsertIntoDB inserts the CommitComment into the database.
func (c *CommitComment) InsertIntoDB() (int, error) {
	if c.Id != 0 {
		return -1, fmt.Errorf("CommitComment has non-zero ID %d; has it already been inserted?", c.Id)
	}
	res, err := DB.Exec(fmt.Sprintf("INSERT INTO %s (commit,user,timestamp,message) VALUES (?,?,?,?);", TABLE_COMMIT_COMMENTS), c.Commit, c.User, c.Timestamp, c.Message)
	if err != nil {
		return -1, fmt.Errorf("Unable to insert commit comment: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("Unable to get last insert ID: %v", err)
	}
	c.Id = int(id)
	return c.Id, nil
}

// DeleteCommitComment deletes the given CommitComment from the database.
func DeleteCommitComment(id int) error {
	stmt := fmt.Sprintf("DELETE FROM %s WHERE id = ?", TABLE_COMMIT_COMMENTS)
	if _, err := DB.Exec(stmt, id); err != nil {
		return fmt.Errorf("Unable to delete commit comment: %v", err)
	}
	return nil
}

// GetBuildsFromDateRange retrieves all builds which finished in the given date range.
func GetBuildsFromDateRange(start, end time.Time) ([]*Build, error) {
	defer timer.New("GetBuildsFromDateRange").Stop()
	var ids []int
	if err := DB.Select(&ids, fmt.Sprintf("SELECT id FROM %s WHERE started > ? and started < ?", TABLE_BUILDS), float64(start.UTC().Unix()), float64(end.UTC().Unix())); err != nil {
		return nil, fmt.Errorf("Failed to obtain builds from date range: %v", err)
	}
	builds, err := GetBuildsFromDB(ids)
	if err != nil {
		return nil, err
	}
	rv := make([]*Build, 0, len(builds))
	for _, b := range builds {
		rv = append(rv, b)
	}
	return rv, nil
}
