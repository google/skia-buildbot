build:
	go install -v ./go/bodymovin_json_schema

run: build
	bodymovin_json_schema --source $(HOME)/projects/bodymovin/docs/json/animation.json  --alsologtostderr

.PHONY: build run
