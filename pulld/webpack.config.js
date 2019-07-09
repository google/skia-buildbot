const commonBuilder = require('pulito');

module.exports = (env, argv) => {
  return commonBuilder(env, argv, __dirname);
}
