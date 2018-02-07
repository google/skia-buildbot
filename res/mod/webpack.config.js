const { commonBuilder } = require('./webpack.common.js');
const { demoFinder } = require('./webpack.demo-finder.js')

module.exports = demoFinder(__dirname, commonBuilder(__dirname));
