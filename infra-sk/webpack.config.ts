import webpack from 'webpack';
import commonBuilder from './pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => commonBuilder(__dirname, args.mode);

export = configFactory;
