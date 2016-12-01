default:
	go install -v ./go/traceserver
	go install -v ./go/tracetool
	go install -v ./go/importtile
	go install -v ./go/skia_ingestion
	go install -v ./go/difftile
	cp $(GOPATH)/bin/skia_ingestion $(GOPATH)/bin/gold_ingestion
	cp $(GOPATH)/bin/skia_ingestion $(GOPATH)/bin/pdf_ingestion
	
