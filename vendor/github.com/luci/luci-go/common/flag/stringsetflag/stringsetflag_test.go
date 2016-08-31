// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package stringsetflag

import (
	"flag"
	"fmt"
	"os"
)

// Example demonstrates how to use stringlistflag.
func Example() {
	sset := Flag{}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&sset, "color", "favorite color, may be repeated.")
	fs.SetOutput(os.Stdout)

	fs.PrintDefaults()

	// Flag parsing.
	fs.Parse([]string{"-color", "Violet", "-color", "Red", "-color", "Violet"})
	fmt.Printf("Value is: %s\n", sset)

	fmt.Println("Likes Blue:", sset.Data.Has("Blue"))
	fmt.Println("Likes Red:", sset.Data.Has("Red"))

	// Output:
	// -color value
	//     	favorite color, may be repeated.
	// Value is: Red,Violet
	// Likes Blue: false
	// Likes Red: true
}
