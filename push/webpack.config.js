const { addCommon } = require('../common/webpack.common.js');
const { commonBuilder } = require('pulito');

module.exports = addCommon(commonBuilder(__dirname));
