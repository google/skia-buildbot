import { resolve } from 'path';
import webpack from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import commonBuilder from '../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  const config = commonBuilder(__dirname, args.mode);
  config.output!.publicPath = '/dist/';
  config.resolve = config.resolve || {};

  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, '..', 'node_modules')];

  let aliases = [
    { from: resolve(__dirname, 'static') },
    { from: resolve(__dirname, 'build/debugger/debugger.wasm') },
    { from: resolve(__dirname, 'build/debugger/debugger.js') },
    { from: resolve(__dirname, 'build/version.js') },
  ];

  config.plugins!.push(
    new CopyWebpackPlugin(aliases),
  );

  config.node = {
    fs: 'empty',
  };

  return config;
};

export = configFactory;
