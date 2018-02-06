const HtmlWebpackPlugin = require('html-webpack-plugin');

var common = require('../../../res/mod/webpack.common.js');

common.entry.push_selection_sk_demo = './push-selection-sk/push-selection-sk-demo.js';
common.plugins.push(
  new HtmlWebpackPlugin({
    filename: 'push-selection-sk-demo.html',
    template: './push-selection-sk/push-selection-sk-demo.html',
    chunks: ['push_selection_sk_demo'],
  }),
);

module.exports = common;
