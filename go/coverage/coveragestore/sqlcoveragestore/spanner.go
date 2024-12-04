package sqlcoveragestore

var statements_spanner = map[statement]string{
	addFile: `
		INSERT INTO testsuitemapping (file_name, builder_name, test_suite_name)
			VALUES ($1, $2, $3)
		`,
	addBuilder: `
		INSERT INTO testsuitemapping
		SET builder_name=$2, test_suite_name=$3
		WHERE file_name=$1;
		`,
	addTestSuite: `
		UPDATE testsuitemapping
		SET test_suite_name = $3
		WHERE testsuitemapping.file_name = $1
		AND testsuitemapping.builder_name = $2
		`,
	deleteFile: `
		DELETE FROM
			testsuitemapping WHERE file_name=$1 AND builder_name=$2`,
	listTestSuite: `
		SELECT id, file_name, builder_name, test_suite_name, last_modified FROM testsuitemapping
		WHERE file_name=$1 AND builder_name=$2`,
	listAll: `
		SELECT id, file_name, builder_name, test_suite_name, last_modified FROM testsuitemapping`,
	listBuilder: `
		SELECT id, file_name, builder_name, test_suite_name, last_modified FROM testsuitemapping WHERE file_name=$1 AND builder_name=$2`,
}
