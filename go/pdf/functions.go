package pdf

import "os/exec"

func commandFound(e string) bool {
	switch exec.Command(e).Run().(type) {
	case *exec.Error:
		return false
	default:
		return true
	}
}
