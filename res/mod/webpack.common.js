const path = require('path');
const CleanWebpackPlugin = require('clean-webpack-plugin');
const ExtractTextPlugin = require('extract-text-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');

module.exports = {
  entry: {
    // Distinguish between demo and app.
    //
    // For demo search through dirs and look for -demo.hs and -demo.html
    // pages. Fill out both entry and plugins values.
    //
    // For apps the user of common fills in the entry and plugins values?
  },
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: '[name]-bundle.js?[chunkhash]',
    publicPath: '/',
  },
  resolve: {
    alias: {
      'skia-elements': path.resolve(__dirname, '..', '..', 'ap', 'skia-elements'),
      'common': path.resolve(__dirname),
    },
  },
  module: {
    rules: [
      {
        test: /\.css$/,
        use: ExtractTextPlugin.extract({
          use: [
            { loader:'css-loader', },
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
    new CleanWebpackPlugin(['dist']),
  ],
}
