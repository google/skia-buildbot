/** Configuration module for esbuild. */

export default {
  define: {
    // Prevent "global is not defined" errors. See https://github.com/evanw/esbuild/issues/73.
    global: 'window',
  },
};
