// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.


// The commonBuilder function generates a common configuration for webpack.
// You can require() it at the start of your webpack.config.js and then make
// modifications to it from there. Users should at a minimum fill in the entry
// points.  See webpack.config.js in this directory as an example.
//
// Usage:
//    A webpack.config.js can be as simple as:
//
//       const commonBuilder = require('pulito');
//       module.exports = (env, argv) => commonBuilder(env, argv, __dirname);
//
//    For an application you need to add the entry points and associate them
//    with HTML files:
//
//        const commonBuilder = require('pulito');
//        const HtmlWebpackPlugin = require('html-webpack-plugin');
//
//        module.exports = (env, argv) => {
//          let common = commonBuilder(env, argv, __dirname);
//          common.entry.index = './pages/index.js'
//          common.plugins.push(
//              new HtmlWebpackPlugin({
//                filename: 'index.html',
//                template: './pages/index.html',
//                chunks: ['index'],
//              })
//          );
//        }
//
//    Note that argv.mode will be set to either 'production', 'development',
//    or '' depending on the --mode flag passed to the webpack cli.
//
// You do not need to add any of the plugins or loaders used here to your
// local package.json, on the other hand, if you add new loaders or plugins
// in your local project then you should 'npm add' them to your local
// package.json.
//
//     build:
//        npx webpack --mode=development
//
//     release:
//        npx webpack --mode=production
//
const { glob } = require('glob');
const path = require('path');
const fs = require('fs')
const { basename, join, resolve } = require('path')
const CleanWebpackPlugin = require('clean-webpack-plugin');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');

const minifyOptions = {
  caseSensitive: true,
  collapseBooleanAttributes: true,
  collapseWhitespace: true,
  // this handles CSS minification in the .js files. For options
  // involving minifying .[s]css files, see ./**/postcss.config.js
  minifyCSS: true,
  minifyJS: true,
  minifyURLS: true,
  removeOptionalTags: true,
  removeRedundantAttributes: true,
  removeScriptTypeAttributes: true,
  removeStyleLinkTypeAttributes: true,
};


/* A function that will look at all subdirectories of 'dir'/modules
 * and adds all demo pages it finds for custom elements it finds there.
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
 */
function demoFinder(dir, webpack_config) {
  // Look at all sub-directories of dir and if a directory contains
  // both a -demo.html and -demo.js file then add the corresponding
  // entry points and Html plugins to the config.

  // Find all the dirs below 'dir'.
  const isDir = filename => fs.lstatSync(filename).isDirectory()
  const moduleDir = path.resolve(dir, 'modules');
  const dirs = fs.readdirSync(moduleDir).map(name => join(moduleDir, name)).filter(isDir);

  dirs.forEach(d => {
    // Look for both a *-demo.js and *-demo.html file in the directory.
    const files = fs.readdirSync(d);
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


/* A function that will look at all subdirectories of 'dir'/pages
 * and adds entries for each page it finds there.
 *
 * Presumes that each page will have both a JS and an HTML file.
 *
 *    pages/
 *      index.js
 *      index.html
 *      search.js
 *      search.html
 *      ....
 *
 * The function will find those files and do the equivalent
 * of the following to the webpack_config:
 *
 *      webpack_config.entry.['index'] = './pages/index/js';
 *      webpack_config.plugins.push(
 *        new HtmlWebpackPlugin({
 *          filename: 'index.html',
 *          template: './pages/index.html',
 *          chunks: ['index'],
 *        }),
 *      );
 *
 */
function pageFinder(dir, webpack_config, minifyOutput) {
  // Look at all sub-directories of dir and if a directory contains
  // both a -demo.html and -demo.js file then add the corresponding
  // entry points and Html plugins to the config.

  // Find all the dirs below 'dir'.
  const pagesDir = path.resolve(dir, 'pages');
  // Look for all *.js files, for each one look for a matching .html file.
  // Emit into config.
  //
  const pagesJS = glob.sync(pagesDir + '/*.js');

  pagesJS.forEach(pageJS => {
    // Look for both a <filename>.js and <filename>.html file in the directory.
    // Strip off ".js" from end and replace with ".html".
    let pageHTML = pageJS.replace(/\.js$/, '.html');
    if (!fs.existsSync(pageHTML)) {
      console.log("WARNING: A page needs both a *.js and a *.html file.");
      return
    }

    let baseHTML = basename(pageHTML);
    let name = basename(pageJS, '.js');
    webpack_config.entry[name] = pageJS;
    let opts = {
      filename: baseHTML,
      template: pageHTML,
      chunks: [name],
    };
    if (minifyOutput) {
      opts.minify = minifyOptions;
    }
    webpack_config.plugins.push(
      new HtmlWebpackPlugin(opts),
    );
  });

  return webpack_config
}

module.exports = (env, argv, dirname) => {
  // The postcss config file must be named postcss.config.js, so we store the
  // different configs in different dirs.
  let prefix = argv.mode === 'production' ? 'prod' : 'dev';
  // This file handles minification, auto-prefixing, etc. See there for configuring
  // those plugins.
  let postCssConfig = path.resolve(__dirname, prefix, 'postcss.config.js');
  let common = {
    entry: {
      // Users of webpack.common must fill in the entry point(s).
    },
    output: {
      path: path.resolve(dirname, 'dist'),
      filename: '[name]-bundle.js?[chunkhash]',
    },
    devServer: {
      contentBase: path.join(__dirname, 'dist')
    },
    module: {
      rules: [
        {
          test: /\.[s]?css$/,
          use: [
            {
              loader: MiniCssExtractPlugin.loader,
              options: {},
            },
            {
              loader: 'css-loader',
              options: {
                importLoaders: 2, // postcss-loader and sass-loader.
              },
            },
            {
              loader: 'postcss-loader',
              options: {
                config: {
                  path: postCssConfig,
                },
              },
            },
            {
              loader: 'sass-loader', // Since SCSS is a superset of CSS we can always apply this loader.
              options: {
                includePaths: [__dirname],
              }
            }
          ],
        },
        {
          test: /\.html$/,
          use: [
            {
              loader:'html-loader',
              options: {
                name: '[name].[ext]',
              },
            }
          ],
        },
      ]
    },
    plugins: [
      new MiniCssExtractPlugin({
        filename: '[name]-bundle.css?[hash]',
      }),
      new CleanWebpackPlugin(
        ['dist'],
        {
          root: path.resolve(dirname),
        }
      ),
      // Users of pulito can append any plugins they want, but they
      // need to make sure they installed them in their project via npm.
    ],
  };
  common = pageFinder(dirname, common, argv.mode === 'production');
  if (argv.mode !== 'production') {
    common = demoFinder(dirname, common);
  }
  return common;
};
