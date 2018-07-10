// A function that modifies a webpack config so that
// common-sk and infra-sk are resolved to the local
// checkout.
const { resolve } = require('path')

module.exports = (config) => {
  config.resolve = config.resolve || {};
  config.resolve.alias = config.resolve.alias || {};
  config.resolve.alias['infra-sk'] = resolve(__dirname, '../infra-sk/');
  config.resolve.alias['common-sk'] = resolve(__dirname, '../common-sk/');
  config.resolve.modules = [resolve(__dirname, 'node_modules'), 'node_modules'];
  return config
}
