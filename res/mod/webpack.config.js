const { commonBuilder } = require('./webpack.common.js');
const { demoFinder } = require('./webpack.demo-finder.js')
const { glob } = require('glob');

let common = demoFinder(__dirname, commonBuilder(__dirname));
common.entry.testable = glob.sync('./tests/*.js');

module.exports = common;
