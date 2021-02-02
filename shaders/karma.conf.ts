import webpack from 'webpack';
import karma from 'karma';
import webpackConfigFactory from './webpack.config';
import { setCommonConfigOptions } from '../infra-sk/karma.common';

export = function(karmaConfig: karma.Config) {
  const webpackConfig = webpackConfigFactory('', { mode: 'development' }) as webpack.Configuration;
  setCommonConfigOptions(karmaConfig, webpackConfig);
}
