package linenumbers

import (
	"fmt"
	"strings"
)

// LineNumbers adds #line numbering to the user's code.
func LineNumbers(c string) string {
	lines := strings.Split(c, "\n")
	ret := []string{}
	for i, line := range lines {
		ret = append(ret, fmt.Sprintf("#line %d", i+1))
		ret = append(ret, line)
	}
	return strings.Join(ret, "\n")
}
