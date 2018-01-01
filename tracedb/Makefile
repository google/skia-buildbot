default:
	go install -v ./go/traceserver
	go install -v ./go/tracetool
	go install -v ./go/importtile
	go install -v ./go/skia_ingestion
	go install -v ./go/difftile

	# Make local copies of the binaries. It makes running locally easier.
	mv $(GOPATH)/bin/gold_ingestion          $(GOPATH)/bin/gold_ingestion_old
	cp $(GOPATH)/bin/skia_ingestion $(GOPATH)/bin/gold_ingestion
	rm $(GOPATH)/bin/gold_ingestion_old

	mv $(GOPATH)/bin/pdfium_gold_ingestion   $(GOPATH)/bin/pdfium_gold_ingestion_old
	cp $(GOPATH)/bin/skia_ingestion $(GOPATH)/bin/pdfium_gold_ingestion
	rm $(GOPATH)/bin/pdfium_gold_ingestion_old

	mv $(GOPATH)/bin/chromevr_gold_ingestion $(GOPATH)/bin/chromevr_gold_ingestion_old
	cp $(GOPATH)/bin/skia_ingestion $(GOPATH)/bin/chromevr_gold_ingestion
	rm $(GOPATH)/bin/chromevr_gold_ingestion_old
