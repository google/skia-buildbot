const path = require('path');
const { demoFinder } = require('../../../res/mod/webpack.demo-finder.js')
const { commonBuilder } = require('../../../res/mod/webpack.common.js');

let common = commonBuilder(__dirname);

// Use finder to auto-populate the demos into the webpack_config.
common = demoFinder(__dirname, common);

module.exports = common;
