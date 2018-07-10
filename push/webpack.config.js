const commonBuilder = require('pulito');
const { resolve } = require('path')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  return config
}
