package parsers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTSImports_Success(t *testing.T) {

	const source = `/* Sample TypeScript file with imports. */
import 'path/to/a';                                // This comment should be ignored.
import "path/to/b";                                // This comment should be ignored.
import * from 'path/to/c';                         // This comment should be ignored.
import * from "path/to/d";                         // This comment should be ignored.
export * from 'path/to/e';                         // This comment should be ignored.
export * from "path/to/f";                         // This comment should be ignored.
import * as foo from 'path/to/g';                  // This comment should be ignored.
import * as foo from "path/to/h";                  // This comment should be ignored.
import { foo_bar, $ } from 'path/to/i';            // This comment should be ignored.
import { foo_bar, $ } from "path/to/j";            // This comment should be ignored.
import { foo as bar } from 'path/to/k';            // This comment should be ignored.
import { foo as bar } from "path/to/l";            // This comment should be ignored.
import foo, { bar, baz as qux } from 'path/to/m';  // This comment should be ignored.
import foo, { bar, baz as qux } from "path/to/n";  // This comment should be ignored.
import {                                           // This comment should be ignored.
  foo,                                             // This comment should be ignored.
  bar,                                             // This comment should be ignored.
} from 'path/to/o';                                // This comment should be ignored.
import {                                           // This comment should be ignored.
  foo,                                             // This comment should be ignored.
  bar,                                             // This comment should be ignored.
} from "path/to/p";                                // This comment should be ignored.

// CSS and Sass imports should be ignored.
import 'styles/a.css';
import 'styles/b.scss';

// Duplicate imports should be ignored.
import 'path/to/a';

// Line comments should be ignored.
//
// import 'line-comment/a';
// import {
//   foo,
//   bar,
// } from 'line-comment/b';

// Block comments should be ignored.
/*
import 'block-comment/a';
import * from 'block-comment/b';
*/

// A more complex block comment.
thisWillBeIgnored(); /*
import 'block-comment/c';
import * from 'block-comment/d'; */ import 'path/to/q';  // This import should NOT be ignored.

// A block comment that starts and ends on the same line.
import /* 'block-comment/e' */ 'path/to/r':

// Tests for various edge cases. Some of these are invalid TS because import is a reserved keyword.
importfrom('ignored/a');
import = 'from "ignored/b"';
import['from "ignored/c"']();
import.from = 'ignored/d';
import.from('ignored/e');
from('ignored/f');
from = 'ignored/g';
"import 'ignored/h'";
`

	expected := []string{
		"path/to/a",
		"path/to/b",
		"path/to/c",
		"path/to/d",
		"path/to/e",
		"path/to/f",
		"path/to/g",
		"path/to/h",
		"path/to/i",
		"path/to/j",
		"path/to/k",
		"path/to/l",
		"path/to/m",
		"path/to/n",
		"path/to/o",
		"path/to/p",
		"path/to/q",
		"path/to/r",
	}

	require.Equal(t, expected, ParseTSImports(source))
}

func TestParseSassImports_Success(t *testing.T) {

	const source = `/* Sample Sass file with @import, @use and @forward statements. */
@import 'path/to/a';  // This comment should be ignored.
@import "path/to/b";  // This comment should be ignored.

@use 'path/to/c';         // This comment should be ignored.
@use "path/to/d";         // This comment should be ignored.
@use 'path/to/e' as foo;  // This comment should be ignored.
@use "path/to/f" as foo;  // This comment should be ignored.
@use 'path/to/g' with (   // This comment should be ignored.
  $foo: 1px,              // This comment should be ignored.
  $bar: #222              // This comment should be ignored.
);                        // This comment should be ignored.
@use "path/to/h" with (   // This comment should be ignored.
  $foo: 1px,              // This comment should be ignored.
  $bar: #222              // This comment should be ignored.
);                        // This comment should be ignored.

@forward 'path/to/i';                  // This comment should be ignored.
@forward "path/to/j";                  // This comment should be ignored.
@forward 'path/to/k' as foo-*;         // This comment should be ignored.
@forward "path/to/l" as foo-*;         // This comment should be ignored.
@forward 'path/to/m' hide $foo, $bar;  // This comment should be ignored.
@forward "path/to/n" hide $foo, $bar;  // This comment should be ignored.
@forward 'path/to/o' with (            // This comment should be ignored.
  $foo: 1px,                           // This comment should be ignored.
  $bar: #222                           // This comment should be ignored.
)                                      // This comment should be ignored.
@forward "path/to/p" with (            // This comment should be ignored.
  $foo: 1px,                           // This comment should be ignored.
  $bar: #222                           // This comment should be ignored.
)                                      // This comment should be ignored.

// Plain CSS imports should be ignored.
@import "theme.css";
@import "http://fonts.googleapis.com/css?family=Droid+Sans";
@import url(theme);
@import "landscape.css" screen and (orientation: landscape);

// Depending on a CSS file with @use or @forward is allowed.
@use "path/to/q.css"
@forward "path/to/r.css";

// Duplicate imports should be ignored.
@import 'path/to/a';

// Line comments should be ignored.
//
// @import 'line-comment/a';
// @use 'line-comment/b' with (
//   $foo: 1px,
//   $bar: #222
// );

// Block comments should be ignored.
/*
@import 'block-comment/a';
@use 'block-comment/b';
@forward 'block-comment/c';
*/

// A more complex block comment.
.this-will-be-ignored {} /*
@import 'block-comment/d';
@use 'block-comment/e';
@forward 'block-comment/f'; */ @import 'path/to/s';  // This import should NOT be ignored.

// A block comment that starts and ends on the same line.
@import /* 'block-comment/g' */ 'path/to/t':
`

	expected := []string{
		"path/to/a",
		"path/to/b",
		"path/to/c",
		"path/to/d",
		"path/to/e",
		"path/to/f",
		"path/to/g",
		"path/to/h",
		"path/to/i",
		"path/to/j",
		"path/to/k",
		"path/to/l",
		"path/to/m",
		"path/to/n",
		"path/to/o",
		"path/to/p",
		"path/to/q.css",
		"path/to/r.css",
		"path/to/s",
		"path/to/t",
	}

	require.Equal(t, expected, ParseSassImports(source))
}
