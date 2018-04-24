// pushk pushes a new version of an app.
//
// Actually just modifies kubernetes yaml files
// with the correct tag for the given image.
//
// pushk docserver
// pushk --rollback docserver
// pushk --project=skia-public docserver
// pushk --rollback --project=skia-public docserver
// pushk --kubefiles=../kube --rollback --project=skia-public docserver
package main

import "fmt"

func main() {
	fmt.Println("vim-go")
}
