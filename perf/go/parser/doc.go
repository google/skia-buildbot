// Lexer and parser for expressions of the form:
//
//   f(g(h("foo"), i(3, "bar")))
//
// Note that while it does understand strings and numbers, it doesn't
// do binary operators. We can do those via functions if needed, ala
// add(x, y), sub(x, y), etc.
//
// Caveats:
// * Only handles ASCII.
//
// For context on how this was written please watch:
//    https://www.youtube.com/watch?v=HxaD_trXwRE
//
package parser
