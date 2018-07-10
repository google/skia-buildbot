const commonBuilder = require('pulito');
const { resolve } = require('path')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.resolve = config.resolve || {};
  config.resolve.alias = config.resolve.alias || {};
  config.resolve.alias['infra-sk'] = resolve(__dirname, '../infra-sk/');
  config.resolve.modules = [resolve(__dirname, 'node_modules'), 'node_modules'];
  return config
}
