import glob from 'glob';
import webpack from 'webpack';
import commonBuilder from '../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  const config = commonBuilder(__dirname, args.mode);
  (config.entry as webpack.Entry)['tests'] = glob.sync('./modules/**/*_test.js');
  return config;
};

export = configFactory;
