See ap/skia-elements/README.md.

Importing
=========

Code from this library should imported under the 'elements-sk' name.
See `pages/index.js` as an example.

If loaded via npm then importing will just work, i.e.:

    import 'elements-sk/checkbox-sk'

If not loaded via npm then the `ap` directory will need to be
added in your `webpack.config.js` under [resolve.modules](https://webpack.js.org/configuration/resolve/#resolve-modules),
i.e.:

    module.exports = {
      ...,
      resolve: {
        modules: [path.resolve(__dirname, '..', '..', 'ap'), 'node_modules'],
      },
