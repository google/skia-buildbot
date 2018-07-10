This is the common set of custom elements used by Skia Infrastructure.
It is built on [common-sk](https://www.npmjs.com/package/common-sk) and [elements-sk](https://www.npmjs.com/package/elements-sk) using [pulito](https://www.npmjs.com/package/pulito)

There is [documentation for each element](https://jsdoc.skia.org).

Use
===

To use this library you should add an alias to your webpack config and
fix the module resolution:

```
const { resolve } = require('path')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);

  config.resolve = config.resolve || {};
  config.resolve.alias = config.resolve.alias || {};
  config.resolve.alias['infra-sk'] = resolve(__dirname, '../infra-sk/');
  config.resolve.modules = [resolve(__dirname, 'node_modules'), 'node_modules'];

  return config;
}
```

The above is needed to work around how webpack resolves modules if all
of your modules don't come from npm. I.e. webpack automatically looks
for a 'node_modules' subdirectory to look for imports. This is no good
for infra-sk which may have a different version of 'elements-sk' than
your application uses, so we need to tell webpack to look in your
apps 'node_modules' first when resolving modules. See:
https://webpack.js.org/configuration/resolve/#resolve-modules
for more details.

If you don't do the above then you can import two different versions
of an element, say 'toast-sk', and then you get the following error:

    Failed to execute 'define' on 'CustomElementRegistry': this name has
    already been used with this registry.

Disclaimer
==========

This is not an officially supported Google product.
