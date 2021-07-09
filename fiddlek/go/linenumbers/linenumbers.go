package linenumbers

// LineNumbers adds #line numbering to the user's code.
func LineNumbers(c string) string {
	return "#line 1\n" + c
}
