const path = require('path');
const CleanWebpackPlugin = require('clean-webpack-plugin');
const ExtractTextPlugin = require('extract-text-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');

module.exports = {
  entry: {
    push_selection_sk_demo: './push-selection-sk/push-selection-sk-demo.js',
  },
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: '[name]-bundle.js?[chunkhash]',
    publicPath: '/',
    library: 'Demo',
  },
  resolve: {
    alias: {
      'skia-elements': path.resolve(__dirname, '..', '..', '..', 'ap', 'skia-elements'),
      'common': path.resolve(__dirname, '..', '..', '..', 'res', 'mod'),
    },
    modules: [path.resolve(__dirname), "node_modules"],
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
    new HtmlWebpackPlugin({
      filename: 'push-selection-sk-demo.html',
      template: './push-selection-sk/push-selection-sk-demo.html',
      chunks: ['push_selection_sk_demo'],
    }),
    new CleanWebpackPlugin(['dist']),
  ],
}
