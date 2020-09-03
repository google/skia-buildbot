import resolve from '@rollup/plugin-node-resolve';
import commonjs from '@rollup/plugin-commonjs';
import sourcemaps from 'rollup-plugin-sourcemaps';

/**
 * An ad-hoc Rollup plugin that comments out any import statements in JS code where the module name
 * ends with ".css" or ".scss".
 *
 * Without this, Rollup chokes when it sees Webpack-style imports such as:
 *
 *   import './styles.scss';
 *
 * See https://rollupjs.org/guide/en/#plugin-development for reference.
 */
function commentOutCssImports() {
  return {
    name: 'comment-out-css-imports',
    transform: function(code) {
      const output =
        code.replace(
          /import.*['"].*\.(css|scss)['"].*/g,
          (match) => `// /* Commented out by Rollup. */ ${match}`);

      return {
        code: output,
        // Reuse existing sourcemap. See https://rollupjs.org/guide/en/#source-code-transformations.
        map: null,
      };
    },
  };
}

export default {
  plugins: [
    resolve(),
    commonjs(),
    sourcemaps(),
    commentOutCssImports(),
  ],
};
