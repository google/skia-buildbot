package sqlite3

import (
	"time"

	"github.com/GeertJohan/go.rice/embedded"
)

func init() {

	// define files
	file2 := &embedded.EmbeddedFile{
		Filename:    "0001_create_initial_tables.down.sql",
		FileModTime: time.Unix(1584451699, 0),

		Content: string("DROP TABLE TraceIDs;\nDROP TABLE SourceFiles;\nDROP TABLE Postings;\nDROP TABLE TraceValues;\nDROP TABLE Shortcuts;\nDROP TABLE Alerts;\nDROP TABLE Regressions;"),
	}
	file3 := &embedded.EmbeddedFile{
		Filename:    "0001_create_initial_tables.up.sql",
		FileModTime: time.Unix(1584451699, 0),

		Content: string("-- This table is used to store trace names. See go/tracestore/sqltracestore.\nCREATE TABLE IF NOT EXISTS TraceIDs  (\n\ttrace_id INTEGER PRIMARY KEY,\n\ttrace_name TEXT UNIQUE NOT NULL       -- The trace name as a structured key, e.g. \",arch=x86,config=8888,\"\n);\n\n-- This table is used to store an inverted index for trace names. See go/tracestore/sqltracestore.\nCREATE TABLE IF NOT EXISTS Postings  (\n\ttile_number INTEGER,                   -- A types.TileNumber.\n\tkey_value text NOT NULL,               -- A key value pair from a structured key, e.g. \"config=8888\".\n\ttrace_id INTEGER,                      -- Id of the trace name from TraceIDS.\n\tPRIMARY KEY (tile_number, key_value, trace_id)\n);\n\n-- This table is used to store source filenames. See go/tracestore/sqltracestore.\nCREATE TABLE IF NOT EXISTS SourceFiles (\n\tsource_file_id INTEGER PRIMARY KEY,\n\tsource_file TEXT UNIQUE NOT NULL     -- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json\n);\n\n-- This table is used to store trace values. See go/tracestore/sqltracestore.\nCREATE TABLE IF NOT EXISTS TraceValues (\n\ttrace_id INTEGER,                    -- Id of the trace name from TraceIDS.\n\tcommit_number INTEGER,               -- A types.CommitNumber.\n\tval REAL,                            -- The floating point measurement.\n\tsource_file_id INTEGER,              -- Id of the source filename, from SourceFiles.\n\tPRIMARY KEY (trace_id, commit_number)\n);\n\n-- This table is used to store shortcuts. See go/shortcut/sqlshortcutstore.\nCREATE TABLE IF NOT EXISTS Shortcuts (\n\tid TEXT UNIQUE NOT NULL PRIMARY KEY,\n\ttrace_ids TEXT                       -- A shortcut.Shortcut serialized as JSON.\n);\n\n-- This table is used to store alerts. See go/alerts/sqlalertstore.\nCREATE TABLE IF NOT EXISTS Alerts (\n\tid INTEGER PRIMARY KEY,\n\talert TEXT,                      -- An alerts.Alert serialized as JSON.\n\tconfig_state INTEGER DEFAULT 0,  -- The Alert.State which is an alerts.ConfigState value.\n\tlast_modified INTEGER            -- Unix timestamp.\n);\n\n-- This table is used to store regressions. See go/regression/sqlregressionstore.\nCREATE TABLE IF NOT EXISTS Regressions (\n\tcommit_number INTEGER,             -- The commit_number where the regression occurred.\n\talert_id INTEGER,                  -- The id of an Alert, i.e. the id from the Alerts table.\n\tregression TEXT,                   -- A regression.Regression serialized as JSON.\n\tPRIMARY KEY (commit_number, alert_id)\n);"),
	}

	// define dirs
	dir1 := &embedded.EmbeddedDir{
		Filename:   "",
		DirModTime: time.Unix(1584451699, 0),
		ChildFiles: []*embedded.EmbeddedFile{
			file2, // "0001_create_initial_tables.down.sql"
			file3, // "0001_create_initial_tables.up.sql"

		},
	}

	// link ChildDirs
	dir1.ChildDirs = []*embedded.EmbeddedDir{}

	// register embeddedBox
	embedded.RegisterEmbeddedBox(`../../../../migrations/sqlite3`, &embedded.EmbeddedBox{
		Name: `../../../../migrations/sqlite3`,
		Time: time.Unix(1584451699, 0),
		Dirs: map[string]*embedded.EmbeddedDir{
			"": dir1,
		},
		Files: map[string]*embedded.EmbeddedFile{
			"0001_create_initial_tables.down.sql": file2,
			"0001_create_initial_tables.up.sql":   file3,
		},
	})
}
