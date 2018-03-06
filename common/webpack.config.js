const { glob } = require('glob');

const { commonBuilder } = require('pulito');

const { addCommon } = require('./webpack.common.js');

let config = addCommon(commonBuilder(__dirname));

config.entry.tests = glob.sync('./modules/**/*_test.js');

module.exports = config;
