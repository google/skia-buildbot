package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testTable struct {
	TableOne   []string `sql_backup:"daily"`
	TableTwo   []string `sql_backup:"weekly"`
	TableThree []string `sql_backup:"daily"`
	TableFour  []string `sql_backup:"monthly"`
	TableFive  []string `sql_backup:"daily"`
	TableSix   []string `sql_backup:"none"`
}

func TestGetSchedules_AllCadencesSet_Success(t *testing.T) {

	r := &fakeRNG{n: 2}

	schedules := getSchedules(testTable{}, "test-backups", "testdbname", r)
	assert.Equal(t, `CREATE SCHEDULE testdbname_daily
FOR BACKUP TABLE testdbname.TableOne, testdbname.TableThree, testdbname.TableFive
INTO 'gs://test-backups/testdbname/daily?AUTH=implicit'
  RECURRING '3 8 * * *'
  FULL BACKUP ALWAYS WITH SCHEDULE OPTIONS ignore_existing_backups;
CREATE SCHEDULE testdbname_weekly
FOR BACKUP TABLE testdbname.TableTwo
INTO 'gs://test-backups/testdbname/weekly?AUTH=implicit'
  RECURRING '5 5 * * 0'
  FULL BACKUP ALWAYS WITH SCHEDULE OPTIONS ignore_existing_backups;
CREATE SCHEDULE testdbname_monthly
FOR BACKUP TABLE testdbname.TableFour
INTO 'gs://test-backups/testdbname/monthly?AUTH=implicit'
  RECURRING '7 4 10 * *'
  FULL BACKUP ALWAYS WITH SCHEDULE OPTIONS ignore_existing_backups;
`, schedules)
}

type tableMissingFrequency struct {
	TableOne []string `sql_backup:"daily"`
	TableTwo []string
}

func TestGetSchedules_MissingCadence_Panics(t *testing.T) {

	assert.Panics(t, func() {
		getSchedules(tableMissingFrequency{}, "test-backups", "testdbname", &fakeRNG{})
	})
}

type fakeRNG struct {
	n int
}

func (f *fakeRNG) Intn(n int) int {
	f.n++
	return f.n % n
}
