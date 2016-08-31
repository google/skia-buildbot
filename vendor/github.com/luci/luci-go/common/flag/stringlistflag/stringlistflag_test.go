// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package stringlistflag

import (
	"flag"
	"fmt"
	"os"
)

// Example demonstrates how to use stringlistflag.
func Example() {
	list := Flag{}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&list, "color", "favorite color, may be repeated.")
	fs.SetOutput(os.Stdout)

	fs.PrintDefaults()

	// Flag parsing.
	fs.Parse([]string{"-color", "Violet", "-color", "Red", "-color", "Violet"})
	fmt.Printf("Value is: %s\n", list)

	// Output:
	// -color value
	//     	favorite color, may be repeated.
	// Value is: Violet, Red, Violet
}
