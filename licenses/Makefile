go-licenses:
	go install github.com/google/go-licenses

check: go-licenses
	./check.sh

regenerate: go-licenses
	./check.sh regenerate
