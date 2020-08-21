(function() {
  require("valid_import");
  // foo
  // bar
  // path to baz
  // concatenated path
  // single quotes
  // no semicolon
  // indented with tabs
  // not indented
  console.log('hello'); require('sandwiched.css'); console.log('bye');
  // require('commented_out.css')
  console.log(`require('inside_a_string.css');`);
  require("another_valid_import");
})();