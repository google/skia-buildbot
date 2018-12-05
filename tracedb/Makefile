KGO := CGO_ENABLED=0 GOOS=linux go build

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

.PHONY: k8s-binaries
k8s-binaries:
	mkdir -p ./build
	rm -f ./build/*
	$(KGO) -o build/traceserver_k8s  -a ./go/traceserver/main.go
	$(KGO) -o build/skia_ingestion_k8s  -a ./go/skia_ingestion/main.go

.PHONY: k8s-release-ingestion
k8s-release-ingestion: k8s-binaries
	./k8s_push_ingestion

.PHONY: k8s-release-traceserver
k8s-release-traceserver: k8s-binaries
	./k8s_push_traceserver
