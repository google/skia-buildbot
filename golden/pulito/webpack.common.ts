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

// The buildCommonWebpackConfig() function generates a common configuration for
// Webpack. You can include it at the start of your webpack.config.ts and then
// make modifications to it from there.
//
// Entry points for pages under the "pages" subdirectory are added to the
// configuration by default, as well as demo pages for modules under the
// "modules" subdirectory if running in development mode.
//
// Users can add any other entry points as needed. See webpack.config.ts in
// this directory as an example.
//
// A webpack.config.ts can be as simple as:
//
//   import buildCommonWebpackConfig from 'pulito';
//   import * as webpack from 'webpack';
//
//   const configFactory: webpack.ConfigurationFactory =
//       (_, args) => buildCommonWebpackConfig(__dirname, args.mode);
//
//   export = configFactory;
//
// An application that requires additional entry points might define them as
// follows:
//
//   import buildCommonWebpackConfig from 'pulito';
//   import * as webpack from 'webpack';
//   import HtmlWebpackPlugin from 'html-webpack-plugin';
//
//   const configFactory: webpack.ConfigurationFactory = (_, args) => {
//     const config = buildCommonWebpackConfig(__dirname, args.mode);
//     (config.entry as webpack.Entry)['index'] = './pages/index.js'
//     config.plugins.push(
//         new HtmlWebpackPlugin({
//           filename: 'index.html',
//           template: './pages/index.html',
//           chunks: ['index'],
//         })
//     );
//     return config;
//   }
//
//   export = configFactory;
//
// To build the application please run one of the following commands:
//
//   # Includes both the application and demo pages.
//   $ npx webpack --mode=development
//
//   # Only includes the application pages.
//   $ npx webpack --mode=production
//
// Notes:
//   - args.mode will be set to either "production", "development", or ""
//     depending on the --mode flag passed to the "npx webpack" command.
//   - Applications do not need to include in their package.json file any of
//     the plugins and loaders used in this file, as those dependencies are
//     satisfied in Pulito's own package.json file.
//   - Any other plugins or loaders used in an application's webpack.config.ts
//     file should be declared as dependencies in that application's
//     package.json file.

import * as webpack from 'webpack';
import * as glob from 'glob';
import * as path from 'path';
import * as fs from 'fs';
import { CleanWebpackPlugin } from 'clean-webpack-plugin';
import MiniCssExtractPlugin from 'mini-css-extract-plugin';
import HtmlWebpackPlugin from 'html-webpack-plugin';
import 'webpack-dev-server';

/** Represents an HTML file and a companion TypeScript or JavaScript file. */
interface HtmlAndTsOrJsFilePair {
  html: string,
  tsOrJs: string,
};

/**
 * Finds all HTML/TypeScript and HTML/JavaScript file pairs with the same base name in the given
 * directory.
 *
 * Prints out an error if an HTML file is found without a companion TypeScript or JavaScript file,
 * or if both are found. Any such HTML files are be excluded from the results.
 */
function findHtmlAndTsOrJsFilePairs(directory: string, htmlGlob = '*.html'): HtmlAndTsOrJsFilePair[] {
  const pagesFound: HtmlAndTsOrJsFilePair[] = [];

  const htmlFiles = glob.sync(path.resolve(directory, htmlGlob));
  htmlFiles.forEach(htmlFile => {
    const tsFile = htmlFile.replace(/\.html$/, '.ts');
    const jsFile = htmlFile.replace(/\.html$/, '.js');

    const tsFileExists = fs.existsSync(tsFile);
    const jsFileExists = fs.existsSync(jsFile);

    // Fail if neither a TypeScript nor a JavaScript file is provided.
    if (!tsFileExists && !jsFileExists) {
      console.log(`WARNING: Page ${htmlFile} needs either a ${tsFile} or a ${jsFile} file.`);
      return;
    }

    // Fail if both a TypeScript and a JavaScript file are provided.
    if (tsFileExists && jsFileExists) {
      console.log(`WARNING: Page ${htmlFile} cannot have both ${tsFile} and ${jsFile} files.`);
      return;
    }

    pagesFound.push({
      html: htmlFile,
      tsOrJs: tsFileExists ? tsFile : jsFile
    });
  });

  return pagesFound;
}

// Production minification settings for HTML pages.
const minifyOptions: HtmlWebpackPlugin.MinifyOptions = {
  caseSensitive: true,
  collapseBooleanAttributes: true,
  collapseWhitespace: true,
  // This handles CSS minification in the .js files. For options involving minifying .[s]css files,
  // see ./**/postcss.config.js
  minifyCSS: true,
  minifyJS: true,
  minifyURLs: true,
  removeOptionalTags: true,
  removeRedundantAttributes: true,
  removeScriptTypeAttributes: true,
  removeStyleLinkTypeAttributes: true,
};

/**
 * Looks for pages (consisting of *.html and *.ts or .js file pairs) inside pagesDirectory, and
 * adds them to the Webpack configuration.
 *
 * Each page gets its own entry point and HtmlWebpackPlugin instance in the Webpack configuration.
 */
function addApplicationPages(
    pagesDirectory: string,
    webpackConfig: webpack.Configuration,
    minifyOutput: boolean): void {
  // Find all HTML pages under the "pages" directory, along with their respective TS or JS files.
  findHtmlAndTsOrJsFilePairs(pagesDirectory).forEach(pair => {
    const chunkName = path.basename(pair.html, '.html');

    // Add TypeScript / JavaScript entry point.
    (webpackConfig.entry as webpack.Entry)[chunkName] = pair.tsOrJs;

    // Add output HTML page.
    webpackConfig.plugins!.push(
      new HtmlWebpackPlugin({
        filename: path.basename(pair.html),
        template: pair.html,
        chunks: [chunkName],
        minify: minifyOutput ? minifyOptions : false
      })
    )
  });
}

/**
 * Looks for demo pages (consisting of *-demo.html and *-demo.ts or .js file pairs) inside all
 * subdirectories of modulesRootDir, and adds them to the Webpack configuration.
 *
 * Each page gets its own entry point and HtmlWebpackPlugin instance in the Webpack configuration.
 *
 * Will throw an exception if any modules are found with more than one demo page.
 */
function addDemoPages(modulesRootDir: string, webpackConfig: webpack.Configuration): void {
  // Find all module directories.
  const moduleDirectories =
      fs.readdirSync(modulesRootDir)
          .map(f => path.join(modulesRootDir, f))
          .filter(f => fs.lstatSync(f).isDirectory());

  // We will populate this array with demo pages found in the module directories.
  const demoPages: {moduleName: string, html: string, tsOrJs: string}[] = [];

  // Search for demo pages inside each module. At most 1 demo page per module is allowed.
  moduleDirectories.forEach(moduleDir => {
    const pairs = findHtmlAndTsOrJsFilePairs(moduleDir, "*-demo.html");

    // At most 1 demo page per module.
    if (pairs.length > 1) {
      throw 'Only one demo page is allowed per module: ${directory}';
    }

    // Keep the first and only demo page, or skip if none is found.
    if (pairs.length == 0) return;
    const pair = pairs[0];

    demoPages.push({moduleName: path.basename(moduleDir), html: pair.html, tsOrJs: pair.tsOrJs});
  });

  // Add demo page entry points and HTML plugins to the Webpack configuration.
  demoPages.forEach(page => {
    // Add TypeScript / JavaScript entry point.
    (webpackConfig.entry as webpack.Entry)[page.moduleName] = page.tsOrJs;

    // Add output HTML page.
    webpackConfig.plugins!.push(
      new HtmlWebpackPlugin({
        filename: page.moduleName + '.html',
        template: page.html,
        chunks: [page.moduleName],
      })
    );
  });
}

/**
 * Builds the common Webpack configuration.
 *
 * @param dirname Application's root directory containing the "modules" and "pages" subdirectories.
 * @param mode Mode string as in the CliConfigOptions interface's "mode" field.
 */
function buildCommonWebpackConfig(
    dirname: string,
    mode?: 'development' | 'production' | 'none'): webpack.Configuration {

  // Convenience constants. Defaults to production if e.g. "npx webpack" is invoked without
  // specifying a mode via the --mode fag.
  const devMode = mode == 'development';
  const prodMode = !devMode;

  const configuration: webpack.Configuration  = {
    entry: {
      // Will be populated with application and demo pages.
    },

    resolve: {
      extensions: ['.ts', '.js']
    },

    output: {
      path: path.resolve(dirname, 'dist'),
      filename: '[name]-bundle.js?[chunkhash]',
    },

    devServer: {
      contentBase: path.join(__dirname, 'dist'),

      // The two below options are required to access the dev server from a different host (e.g.
      // serve from workstation, access from laptop).
      host: '0.0.0.0',
      disableHostCheck: true,
    },

    devtool: devMode ? 'inline-source-map' : false,

    mode: devMode ? 'development' : 'production',

    module: {
      rules: [
        {
          test: /\.ts$/,
          use: 'ts-loader',
          exclude: /node_modules/
        },
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
                  // This file handles minification, auto-prefixing, etc. See there for configuing
                  // those plugins.
                  //
                  // The PostCSS config file must be named postcss.config.js, so we store the
                  // different configs in different directories.
                  path: path.resolve(__dirname, devMode ? 'prod' : 'dev', 'postcss.config.js'),
                },
              },
            },
            {
              // Since SCSS is a superset of CSS we can always apply this loader.
              loader: 'sass-loader',
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
      new CleanWebpackPlugin(),
    ]
  };

  // Add application pages.
  addApplicationPages(path.resolve(dirname, 'pages'), configuration, prodMode);

  // Add demo pages.
  if (devMode) {
    addDemoPages(path.resolve(dirname, 'modules'), configuration);
  }

  return configuration;
};

export default buildCommonWebpackConfig;
