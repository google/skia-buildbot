package metrics2

import "testing"

const (
	JSON = `{
		 "status":"success",
		 "data":{
				"resultType":"vector",
				"result":[
					 {
							"metric":{
								 "__name__":"perf_clustering_untriaged",
								 "instance":"skia-perf:20000",
								 "job":"skiaperfd"
							},
							"value":[
								 1487950095.732,
								 "15"
							]
					 }
				]
		 }
	}`
)

func TestStep(t *testing.T) {

}
