package statements

import (
	"bytes"
	"fmt"
	"text/template"
)

type tiledTracesContext struct {
	StartID             int
	EndID               int
	TraceRangeCondition string
}

var tiledTracesTempl = template.Must(template.New("").Parse(`
INSERT INTO TiledTraceDigests
SELECT DISTINCT trace_id, {{ .StartID }}, digest
FROM TraceValues
WHERE {{ .TraceRangeCondition }} AND
commit_id >= {{ .StartID }} AND commit_id < {{ .EndID }}
ON CONFLICT DO NOTHING;`))

func CreateTiledTraceDigestsShard(shard byte, startID, tileWidth int) string {
	ctx := tiledTracesContext{
		StartID: startID,
		EndID:   startID + tileWidth,
	}
	var buf bytes.Buffer
	if shard == 255 {
		ctx.TraceRangeCondition = `trace_id > x'ff'`
		err := tiledTracesTempl.Execute(&buf, ctx)
		if err != nil {
			panic(err) // should never happen
		}
		return buf.String()
	}
	ctx.TraceRangeCondition = fmt.Sprintf(`trace_id > x'%02x' and trace_id < x'%02x'`, shard, shard+1)
	err := tiledTracesTempl.Execute(&buf, ctx)
	if err != nil {
		panic(err) // should never happen
	}
	return buf.String()
}
