import { resolve } from 'path';
import webpack from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import commonBuilder from '../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  const config = commonBuilder(__dirname, args.mode);
  config.output!.publicPath = '/dist/';
  config.resolve = config.resolve || {};
  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, 'node_modules'), 'node_modules'];

  // Copy static assets into the //golden/dist directory.
  const copyPatterns: any[] = [{
    from: resolve(__dirname, 'static/favicon.ico'), to: 'favicon.ico',
  }];
  if (args.mode !== 'production') {
    copyPatterns.push({from: resolve(__dirname, 'demo-page-assets')});
  }
  config.plugins!.push(new CopyWebpackPlugin(copyPatterns));

  return config;
};

export = configFactory;
