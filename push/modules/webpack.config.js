const { demoFinder } = require('../../../res/mod/webpack.demo-finder.js')
const { configBuilder } = require('../../../res/mod/webpack.common.js');

module.exports = demoFinder(__dirname, configBuilder(__dirname));
