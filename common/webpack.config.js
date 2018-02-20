const { glob } = require('glob');

const { configBuilder } = require('pulito');

const { addCommon } = require('./webpack.common.js');

let config = addCommon(configBuilder(__dirname));

config.entry.tests = glob.sync('./tests/*.js');

module.exports = config;
