import webpack from 'webpack';
import commonBuilder from '../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  return commonBuilder(__dirname, args.mode);
};

export = configFactory;
