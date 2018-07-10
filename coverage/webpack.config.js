const commonBuilder = require('pulito');
const commonsk = require('../common-sk/webpack.common.js');

module.exports = (env, argv) => commonsk(commonBuilder(env, argv, __dirname));
