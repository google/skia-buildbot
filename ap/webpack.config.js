const path = require('path');
const CleanWebpackPlugin = require('clean-webpack-plugin');
const ExtractTextPlugin = require('extract-text-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');

module.exports = {
  entry: {
    'index'   : './pages/index.js',
    'icon-sk' : './pages/icon-sk.js',
  },
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: '[name]-bundle.js?[chunkhash]',
  },
  resolve: {
    modules: [path.resolve(__dirname), "node_modules"],
  },
  module: {
    rules: [
      {
        test: /\.css$/,
        use: ExtractTextPlugin.extract({
          use: [
            {
              loader:'css-loader',
              options: {
                // minimize: true,  // Should be turned on in prod.
              },
            },
            { loader:'postcss-loader' },
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
      filename: '[name]-bundle.css?[hash]',
    }),
    new HtmlWebpackPlugin({
      filename: 'index.html',
      template: './pages/index.html',
      chunks: ['index'],
    }),
    new HtmlWebpackPlugin({
      filename: 'icon-sk.html',
      template: './pages/icon-sk.html',
      chunks: ['icon-sk'],
    }),
    new CleanWebpackPlugin(['dist']),
  ],
}
