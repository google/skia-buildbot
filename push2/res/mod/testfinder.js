const finder = require('../../../res/mod/demo-finder.js')

let config = {
  entry: {},
  plugins: [],
};

finder(__dirname, config);
console.log(config);
console.log(config.plugins[0].options);
