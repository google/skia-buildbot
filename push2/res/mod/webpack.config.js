const { demoFinder } = require('../../../res/mod/webpack.demo-finder.js')
const { commonBuilder } = require('../../../res/mod/webpack.common.js');

module.exports = demoFinder(__dirname, commonBuilder(__dirname));
