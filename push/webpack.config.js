const { commonBuilder } = require('pulito');

const { addCommon } = require('../common/webpack.common.js');

module.exports = addCommon(commonBuilder(__dirname));
