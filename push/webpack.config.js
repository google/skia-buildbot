const { configBuilder } = require('pulito');

const { addCommon } = require('../common/webpack.common.js');

module.exports = addCommon(configBuilder(__dirname));
