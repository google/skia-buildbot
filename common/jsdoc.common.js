// This is a common configuration for running jsdoc
// over modules, including custom elements. To use
// this, just create a jsdoc.config.js file in your
// application directory and populate it with:
//
//     module.exports = require('../common/jsdoc.common.js');
//
// Add jsdoc via yarn:
//
//   $ yarn add jsdoc
//
// Then add this to your Makefile:
//
//     docs:
//         npx jsdoc -c jsdoc.config.js `find modules -name "*.js"`
//
// This config loads the element plugin which adds support
// for @evt and @attr tags in documentation.
//
// Docs will appear in the ./out directory, which should be added
// to .gitignore.
const path = require('path');

module.exports = {
  plugins: [path.resolve(__dirname, './plugins/element')],
};
