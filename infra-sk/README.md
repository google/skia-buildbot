This is the common set of custom elements used by Skia Infrastructure.
It is built on [common-sk](https://www.npmjs.com/package/common-sk) and [elements-sk](https://www.npmjs.com/package/elements-sk) using [pulito](https://www.npmjs.com/package/pulito)

There is [documentation for each element](https://jsdoc.skia.org).


Use
===

To use this library you should add the following to your webpack config:

```
const { resolve } = require('path')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.resolve = config.resolve || {};
  config.resolve.modules = [resolve(__dirname, 'node_modules')];
  return config;
}
```

This changes forces module resolution to happen only in the directory where the
project's `webpack.config.js` sits, i.e. the `node_modules` diretory under
`infra_sk` will be ignored, which means that all dependencies for infra-sk have
to exist in the projects `package.json` file.

Disclaimer
==========

This is not an officially supported Google product.
