const path = require('path');
const CleanWebpackPlugin = require('clean-webpack-plugin');
const ExtractTextPlugin = require('extract-text-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');

module.exports = {
  entry: {
    login_demo: './login-demo.js',
  },
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: '[name]-bundle.js?[chunkhash]',
    publicPath: '/',
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
      filename: 'login-demo.html',
      template: './login-demo.html',
      chunks: ['login_demo'],
    }),
    new CleanWebpackPlugin(['dist']),
  ],
}
