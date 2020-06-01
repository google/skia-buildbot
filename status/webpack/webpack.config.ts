import { resolve } from 'path';
import webpack from 'webpack';
import commonBuilder from '../../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  const config = commonBuilder(__dirname, args.mode);

  config.entry = {
    task_driver_sk: './modules/task-driver-sk.js'
  };

  config.output!.path = resolve(__dirname, '../res/imp');
  config.output!.publicPath = '/res/imp';

  // Do not delete the contents of ../res/imp when Webpack runs.
  config.plugins = config.plugins!.filter(p => p.constructor.name != 'CleanWebpackPlugin');

  config.resolve = config.resolve || {};

  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, 'node_modules')];

  return config;
};

export = configFactory;
