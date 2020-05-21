import { resolve } from 'path';
import * as webpack from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import commonBuilder from './pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  const config = commonBuilder(__dirname, args.mode);
  config.output!.publicPath = '/dist/';
  config.resolve = config.resolve || {};
  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, 'node_modules'), 'node_modules'];

  if (args.mode !== 'production') {
    config.plugins!.push(
      new CopyWebpackPlugin([
        {
          from: resolve(__dirname, 'demo-page-assets'),
        },
      ]),
    );
  }

  return config;
}

export = configFactory;
