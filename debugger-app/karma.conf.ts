import webpack from 'webpack';
import karma from 'karma';
import webpackConfigFactory from './webpack.config';
import { setCommonConfigOptions } from '../infra-sk/karma.common';

export = function(karmaConfig: karma.Config) {
  const webpackConfig = webpackConfigFactory('', { mode: 'development' }) as webpack.Configuration;
  setCommonConfigOptions(karmaConfig, webpackConfig);

  // Note that any array type common options set by setCommonConfigOptions must be repeated
  // here because @types/karma only exports Config.set, and while it merges configs, it
  // always replaces arrays

  karmaConfig.set({
    files: [
      // debugger-page-sk.ts expects to find SKIA_VERSION defined
      'build/version.js',

      // out debugger wasm products
      'build/debugger/debugger.js',
      { pattern: 'build/debugger/debugger.wasm', included: false, served: true },

      // the test files that setCommonConfigOptions already set
      'node_modules/@webcomponents/custom-elements/custom-elements.min.js',
      'modules/*_test.ts',
      'modules/**/*_test.ts',
      'modules/*_test.js',
      'modules/**/*_test.js',
    ],

    proxies: {
      '/dist/debugger.wasm': '/base/build/debugger/debugger.wasm',
    },
  });
}
