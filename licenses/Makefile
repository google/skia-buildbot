go-licenses:
	go install github.com/google/go-licenses/v2

check: go-licenses
	./check.sh

regenerate: go-licenses
	./check.sh regenerate
