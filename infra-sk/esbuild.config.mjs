/** Configuration module for esbuild. */

/**
 * An ad-hoc esbuild plugin that ignores any *.scss or *.css imports from JavaScript or TypeScript.
 *
 * Without this, esbuild chokes when it sees Webpack-style imports such as:
 *
 *     import './styles.scss';
 *
 * When this plugin sees a *.scss or *.css import, it tells esbuild to resolve it as an empty
 * JavaScript file, which is effectively the same as pretending it was never there to begin with.
 *
 * We should delete this plugin once we've cleaned up all Webpack-style CSS/SCSS imports from
 * TypeScript files.
 *
 * Inspired by https://github.com/evanw/esbuild/issues/808#issuecomment-806086039.
 *
 * See https://esbuild.github.io/plugins for reference.
 */
const ignoreScssImportsPlugin = {
  name: 'ignore-scss-imports',
  setup(build) {
    build.onResolve({ filter: /\.(scss|css)$/ }, args => ({
      path: args.path,
      namespace: 'ignored-scss-imports-ns',
    }))
    build.onLoad({ filter: /.*/, namespace: 'ignored-scss-imports-ns' }, async (args) => {
      return {
        contents: '',
        loader: 'js',
      }
    })
  },
};

export default {
  define: {
    // Prevent "global is not defined" errors. See https://github.com/evanw/esbuild/issues/73.
    "global": "window",
  },
  plugins: [ignoreScssImportsPlugin],
}
