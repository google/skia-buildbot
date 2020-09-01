(function() {
  require("valid_import");
  // foo
  // foo (plain CSS)
  // bar
  // path to baz
  // concatenated path
  // single quotes
  // no semicolon
  // indented with tabs
  // not indented
  console.log('hello'); require('sandwiched.scss'); console.log('bye');
  // require('commented_out.scss')
  console.log(`require('inside_a_string.scss');`);
  require("another_valid_import");
})();