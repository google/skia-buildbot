(function() {
  require("valid_import");
  require("foo.css");  // foo
  require("./bar.css");  // bar
  require("../path/to/baz.css");  // path to baz
  require("concatenated/" + "path.css");  // concatenated path
  require('single_quotes.css');  // single quotes
  require('no_semicolon.css')  // no semicolon
		require('indented_with_tabs.css')  // indented with tabs
require('not_indented.css')  // not indented
  console.log('hello'); require('sandwiched.css'); console.log('bye');
  // require('commented_out.css')
  console.log(`require('inside_a_string.css');`);
  require("another_valid_import");
})();