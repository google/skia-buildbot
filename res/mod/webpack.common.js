// The commonBuilder function generates a common configuration for webpack.
// You can require() it at the start of your webpack.config.js and then make
// modifications to it from there. Users should at a minimum fill in the entry
// points.  See webpack.config.js in this directory as an example.
//
// Usage:
//    A webpack.config.js can be as simple as:
//
//       const { commonBuilder } = require('../../res/mod/webpack.common.js');
//       let common = commonBuilder(__dirname);
//       module.exports = common;
//
//    If you are just building demo pages for elements:
//
//       const { commonBuilder } = require('./webpack.common.js');
//       const { demoFinder } = require('./webpack.demo-finder.js')
//
//       module.exports = demoFinder(__dirname, commonBuilder(__dirname));
//
//    For an application you need to add the entry points and associate them
//    with HTML files:
//
//        const { commonBuilder } = require('../res/mod/webpack.common.js');
//        const HtmlWebpackPlugin = require('html-webpack-plugin');
//
//        let common = commonBuilder(__dirname);
//        common.entry.index = './pages/index.js'
//        common.plugins.push(
//            new HtmlWebpackPlugin({
//              filename: 'index.html',
//              template: './pages/index.html',
//              chunks: ['index'],
//            })
//        );
//
//        module.exports = common
//
// This config adds aliases for the 'skia-elements' and 'common' libraries,
// so they will always be available at those prefixes. See 'demo/demo.js'.
//
// The alias also works for SCSS imports, but just remember to prepend a '~'
// to the @import filename so webpack knows it should resolve the name. That
// is, to import the colors.scss file from your local scss file just add:
//
//    @import '~common/colors'
//
// You do not need to add any of the plugins or loaders used here to your
// local package.json, on the other hand, if you add new loaders or plugins
// in your local project then you should 'yarn add' them to your local
// package.json.
//
// This config understands NODE_ENV and can build production versions
// of assets by setting the environment variable. An example Makefile:
//
//     build:
//      	npx webpack
//
//     release:
//      	NODE_ENV=production npx webpack
//
//
// NB - Remember to 'yarn add' any plugins or loaders added to this file in
//   this directory so they are available to all users of webpack.common.js.
//
const path = require('path');
const CleanWebpackPlugin = require('clean-webpack-plugin');
const ExtractTextPlugin = require('extract-text-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const MinifyPlugin = require("babel-minify-webpack-plugin");

module.exports.commonBuilder = function(dirname) {
  let common = {
    entry: {
      // Users of webpack.common must fill in the entry point(s).
    },
    output: {
      path: path.resolve(dirname, 'dist'),
      filename: '[name]-bundle.js?[chunkhash]',
      publicPath: '/',
    },
    resolve: {
      alias: {
        // Keep these libraries at well known locations.
        'skia-elements': path.resolve(__dirname, '..', '..', 'ap', 'skia-elements'),
        'common': path.resolve(__dirname),
      },
    },
    resolveLoader: {
      // This config file references loaders, make sure users of this common
      // config can find those loaders by including the local node_modules
      // directory.
      modules: [path.resolve(__dirname, 'node_modules'), 'node_modules'],
    },
    module: {
      rules: [
        {
          test: /\.[s]?css$/,
          use: ExtractTextPlugin.extract({
            use: [
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
                    path: path.resolve(__dirname, 'postcss.config.js')
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
          })
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
      new ExtractTextPlugin({
        filename: '[name]-bundle.css?[contenthash]',
      }),
      new CleanWebpackPlugin(
        ['dist'],
        {
          root: path.resolve(dirname),
        }
      ),
      // Users of webpack.common can append any plugins they want, but they
      // need to make sure they installed them in their project via yarn.
    ],
  };
  if (process.env.NODE_ENV == 'production') {
    common.plugins.push(
      new MinifyPlugin({}, {
        comments: false,
      })
    )
  }
  return common;

};
