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
// This config adds aliases for the 'skia-elements' and 'common' libraries,
// so they will always be available at those prefixes. See 'demo/demo.js'.
//
// You do not need to add any of the plugins or loaders used here to your
// local package.json, on the other hand, if you add new loaders or plugins
// in your local project then you should 'yarn add' them to your local
// package.json.
//
// NB - Remember to 'yarn add' any plugins or loaders added to this file in
//   this directory so they are available to all users of webpack.common.js.
//
// TODO - Add a production option to this config to produce minimized js, css, etc.
const path = require('path');
const CleanWebpackPlugin = require('clean-webpack-plugin');
const ExtractTextPlugin = require('extract-text-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');


module.exports.commonBuilder = function(dirname) {
  return {
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
              'sass-loader', // Since SCSS is a superset of CSS we can always apply this loader.
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

};
