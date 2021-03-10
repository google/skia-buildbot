package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
)

const template = `
<html>
  <body>
    <table>
	  
	</table>
  </body>
</html>
`

func main() {
	util.WithReadFile("/home/jcgregorio/alerts.csv", func(r io.Reader) error {
		records, err := csv.NewReader(r).ReadAll()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("id,name")
		for i, record := range records {
			if i == 0 {
				continue
			}
			// id,alert,config_state,last_modified
			var a alerts.Alert
			if err := json.Unmarshal([]byte(record[1]), &a); err != nil {
				sklog.Fatal(err)
			}
			fmt.Printf("%s, %s\n", record[0], a.DisplayName)
		}
		return nil
	})
}
