const HtmlWebpackPlugin = require('html-webpack-plugin');
const { lstatSync, readdirSync } = require('fs')
const { basename, join, resolve } = require('path')

/* Exports a function that will look at all subdirectories of 'dir',
 *
 * Presumes that each element will have a file structure of:
 *
 *    push-selection-sk/
 *      index.js
 *      push-selection-sk-demo.html
 *      push-selection-sk-demo.js
 *      push-selection-sk.css
 *      push-selection-sk.js
 *
 * Where the -demo.html and -demo.js files are only used to demo
 * the element.
 *
 * The function will find those demo files and do the equivalent
 * of the following to the webpack_config:
 *
 *      webpack_config.entry.["pusk-selection-sk"] = './push-selection-sk/push-selection-sk-demo.js';
 *      webpack_config.plugins.push(
 *        new HtmlWebpackPlugin({
 *          filename: 'push-selection-sk.html',
 *          template: './push-selection-sk/push-selection-sk-demo.html',
 *        }),
 *      );
 *
 * */
module.exports.demoFinder = function(dir, webpack_config) {
  // Look at all sub-directories of dir and if a directory contains
  // both a -demo.html and -demo.js file then add the corresponding
  // entry points and Html plugins to the config.

  // Find all the dirs below 'dir'.
  const isDir = filename => lstatSync(filename).isDirectory()
  const dirs = readdirSync(dir).map(name => join(dir, name)).filter(isDir);

  dirs.forEach(d => {
    // Look for both a *-demo.js and *-demo.html file in the directory.
    const files = readdirSync(d);
    let demoHTML = '';
    let demoJS = '';
    files.forEach(file => {
      if (file.endsWith('-demo.html')) {
        if (!!demoHTML) {
          throw 'Only one -demo.html file is allowed per directory: ' + file;
        }
        demoHTML = file;
      }
      if (file.endsWith('-demo.js')) {
        if (demoJS != '') {
          throw 'Only one -demo.js file is allowed per directory: ' + file;
        }
        demoJS = file;
      }
    });
    if (!!demoJS && !!demoHTML) {
      let name = basename(d);
      webpack_config.entry[name] = join(d, demoJS);
      webpack_config.plugins.push(
        new HtmlWebpackPlugin({
          filename: name + '.html',
          template: join(d, demoHTML),
          chunks: [name],
        }),
      );
    } else if (!!demoJS || !!demoHTML) {
      console.log("WARNING: An element needs both a *-demo.js and a *-demo.html file.");
    }
  });

  return webpack_config
}
