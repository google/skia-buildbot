package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	http.Handle("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(os.Stdout, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		if err := r.Body.Close(); err != nil {
			_ = fmt.Fprintf(os.Stdout, "%s\n", err)
		}
	})
	err := http.ListenAndServe(":9900", nil)
	panic(err)
}
