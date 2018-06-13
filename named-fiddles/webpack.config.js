const commonBuilder = require('pulito');
//const commonBuilder = require('../webpack.common.js');

module.exports = (env, argv) => commonBuilder(env, argv, __dirname);
