import { resolve } from 'path';
import webpack from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import commonBuilder from '../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  const config = commonBuilder(__dirname, args.mode);
  config.output!.publicPath = '/dist/';
  config.plugins!.push(
    new CopyWebpackPlugin([
      {
        from: resolve(__dirname, 'res/img/favicon.ico'),
        to: 'favicon.ico',
      },
    ]),
  );
  config.resolve = config.resolve || {};
  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, '..', 'node_modules')];

  return config;
};

export = configFactory;
