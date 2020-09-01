(function() {
  require("valid_import");
  require("foo.scss");  // foo
  require("foo.css");  // foo (plain CSS)
  require("./bar.scss");  // bar
  require("../path/to/baz.scss");  // path to baz
  require("concatenated/" + "path.scss");  // concatenated path
  require('single_quotes.scss');  // single quotes
  require('no_semicolon.scss')  // no semicolon
		require('indented_with_tabs.scss')  // indented with tabs
require('not_indented.scss')  // not indented
  console.log('hello'); require('sandwiched.scss'); console.log('bye');
  // require('commented_out.scss')
  console.log(`require('inside_a_string.scss');`);
  require("another_valid_import");
})();