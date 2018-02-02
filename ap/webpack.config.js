const path = require('path');
const CleanWebpackPlugin = require('clean-webpack-plugin');
const ExtractTextPlugin = require('extract-text-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');

module.exports = {
  entry: {
    index: './pages/index.js',
    users: './pages/users.js',
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
            {
              loader:'css-loader',
              options: {
                minimize: true,
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
      filename: '[name]-bundle.css?[contenthash]',
    }),
    new HtmlWebpackPlugin({
      filename: 'index.html',
      template: './pages/index.html',
      chunks: ['index'],
    }),
    new HtmlWebpackPlugin({
      filename: 'users.html',
      template: './pages/users.html',
      chunks: ['users'],
    }),
    new CleanWebpackPlugin(['dist']),
  ],
}
