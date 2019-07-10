const commonBuilder = require('pulito');
const { resolve } = require('path')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.entry['task_driver_sk'] = './modules/task-driver-sk.js';
  config.output.path = resolve(__dirname, '../res/imp');
  config.output.publicPath='/res/imp/';
  config.resolve = config.resolve || {};
  config.resolve.modules = [resolve(__dirname, 'node_modules')];
  return config
}
