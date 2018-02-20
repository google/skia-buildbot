const { addCommon } = require('./webpack.common.js');
const { commonBuilder } = require('pulito');
const { glob } = require('glob');

let config = addCommon(commonBuilder(__dirname));

config.entry.tests = glob.sync('./tests/*.js');

module.exports = config;
