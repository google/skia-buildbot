import { resolve } from 'path';
import webpack from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import commonBuilder from '../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  const config = commonBuilder(__dirname, args.mode);

  // Make all CSS/JS files appear at the /res location.
  config.output!.publicPath = '/res/';

  config.plugins!.push(
    new CopyWebpackPlugin([
      { from: resolve(__dirname, 'build/canvaskit/canvaskit.wasm') },
      { from: resolve(__dirname, 'build/pathkit/pathkit.wasm') },
      { from: resolve(__dirname, 'node_modules/@webcomponents/custom-elements/custom-elements.min.js') },
    ]),
  );

  config.node = {
    fs: 'empty',
  };

  return config;
};

export = configFactory;
