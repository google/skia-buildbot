package sqltracestore

var spannerTemplates = map[statement]string{
	insertIntoTraceValues: `INSERT INTO
            TraceValues (trace_id, commit_number, val, source_file_id)
        VALUES
        {{ range $index, $element :=  . -}}
            {{ if $index }},{{end}}
            (
                '{{ $element.MD5HexTraceID }}', {{ $element.CommitNumber }}, {{ $element.Val }}, {{ $element.SourceFileID }}
            )
        {{ end }}
        `,
	convertTraceIDs: `
        {{ $tileNumber := .TileNumber }}
        SELECT
            key_value, trace_id
        FROM
            Postings
            {{ .AsOf }}
        WHERE
            tile_number = {{ $tileNumber }}
            AND trace_id IN (
                {{ range $index, $trace_id :=  .TraceIDs -}}
                    {{ if $index }},{{ end -}}
                    '{{ $trace_id }}'
                {{ end -}}
            )
    `,
	queryTraceIDs: `
        {{ $key := .Key }}
        SELECT
            trace_id
        FROM
            Postings
            {{ .AsOf }}
        WHERE
            tile_number = {{ .TileNumber }}
            AND key_value IN
            (
                {{ range $index, $value :=  .Values -}}
                    {{ if $index }},{{end}}
                    '{{ $key }}={{ $value }}'
                {{ end }}
            )
            {{ .RestrictClause }}
		ORDER BY trace_id`,
	queryTraceIDsByKeyValue: `
		{{ $key := .Key }}
		SELECT
			trace_id
		FROM
			Postings
			{{ .AsOf }}
		WHERE
			tile_number = {{ .TileNumber }}
			AND key_value IN
			(
				{{ range $index, $value :=  .Values -}}
					{{ if $index }},{{end}}
					'{{ $key }}={{ $value }}'
				{{ end }}
			)
		ORDER BY trace_id`,
	readTraces: `
        SELECT
            trace_id,
            commit_number,
            val
        FROM
            TraceValues
            {{ .AsOf }}
        WHERE
            commit_number >= {{ .BeginCommitNumber }}
            AND commit_number <= {{ .EndCommitNumber }}
            AND trace_id IN
            (
                {{ range $index, $trace_id :=  .TraceIDs -}}
                    {{ if $index }},{{end}}
                    '{{ $trace_id }}'
                {{ end }}
            )
        `,
	getSource: `
        SELECT
            SourceFiles.source_file
        FROM
            TraceValues
        INNER JOIN SourceFiles ON SourceFiles.source_file_id = TraceValues.source_file_id
        WHERE
            TraceValues.trace_id = '{{ .MD5HexTraceID }}'
            AND TraceValues.commit_number = {{ .CommitNumber }}`,
	insertIntoPostings: `
        INSERT INTO
            Postings (tile_number, key_value, trace_id)
        VALUES
            {{ range $index, $element :=  . -}}
                {{ if $index }},{{end}}
                ( {{ $element.TileNumber }}, '{{ $element.Key }}={{ $element.Value }}', '{{ $element.MD5HexTraceID }}' )
            {{ end }}
        ON CONFLICT (tile_number, key_value, trace_id)
        DO NOTHING`,
	insertIntoParamSets: `
        INSERT INTO
            ParamSets (tile_number, param_key, param_value)
        VALUES
            {{ range $index, $element :=  . -}}
                {{ if $index }},{{end}}
                ( {{ $element.TileNumber }}, '{{ $element.Key }}', '{{ $element.Value }}' )
            {{ end }}
        ON CONFLICT (tile_number, param_key, param_value)
        DO NOTHING`,
	paramSetForTile: `
        SELECT
           param_key, param_value
        FROM
            ParamSets
            {{ .AsOf }}
        WHERE
            tile_number = {{ .TileNumber }}`,
	countMatchingTraces: `
        {{ $key := .Key }}
        SELECT
            count(*)
        FROM (
            SELECT
               *
            FROM
               Postings
            WHERE
               tile_number = {{ .TileNumber }}
               AND key_value IN
               (
                  {{ range $index, $value :=  .Values -}}
                     {{ if $index }},{{end}}
                     '{{ $key }}={{ $value }}'
                  {{ end }}
               )
            LIMIT {{ .CountOptimizationThreshold }}
        ) AS temp`,
	restrictClause: `
    AND trace_ID IN
    ({{ range $index, $value := .Values -}}
            {{ if $index }},{{end}}
            '{{ $value }}'
    {{ end }})`,
}

var spannerStatements = map[statement]string{
	insertIntoSourceFiles: `
        INSERT INTO
            SourceFiles (source_file)
        VALUES
            ($1)
        ON CONFLICT (source_file_id)
        DO NOTHING`,
	getSourceFileID: `
        SELECT
            source_file_id
        FROM
            SourceFiles
        WHERE
            source_file=$1`,
	getLatestTile: `
        SELECT
            tile_number
        FROM
            ParamSets
        ORDER BY
            tile_number DESC
        LIMIT
            1;`,
	traceCount: `
        SELECT
            COUNT(DISTINCT trace_id)
        FROM
            Postings
        WHERE
          tile_number = $1`,
	getLastNSources: `
        SELECT
            SourceFiles.source_file, TraceValues.commit_number
        FROM
            TraceValues
            INNER JOIN
                SourceFiles
            ON
                TraceValues.source_file_id = SourceFiles.source_file_id
        WHERE
            TraceValues.trace_id=$1
        ORDER BY
            TraceValues.commit_number DESC
        LIMIT
            $2`,
	getTraceIDsBySource: `
        SELECT
            Postings.key_value, Postings.trace_id
        FROM
            SourceFiles
            INNE JOIN
                TraceValues
            ON
                TraceValues.source_file_id = SourceFiles.source_file_id
            INNER JOIN
                Postings
            ON
                TraceValues.trace_id = Postings.trace_id
        WHERE
            SourceFiles.source_file = $1
        AND
            Postings.tile_number= $2
        ORDER BY
            Postings.trace_id`,
	countCommitInCommitNumberRange: `
		SELECT
			count(*)
		FROM
			Commits
		WHERE
			commit_number >= $1
			AND commit_number <= $2`,
	getCommitsFromCommitNumberRange: `
		SELECT
			commit_number, git_hash, commit_time, author, subject
		FROM
			Commits
		WHERE
			commit_number >= $1
			AND commit_number <= $2
		ORDER BY
			commit_number ASC
		`,
	deleteCommit: `
		DELETE FROM
			Commits
		WHERE
			commit_number = $1
		`,
}