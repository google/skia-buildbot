import { resolve } from 'path';
import webpack from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import commonBuilder from '../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  const config = commonBuilder(__dirname, args.mode);

  config.output!.publicPath = '/static/';

  config.plugins!.push(
    new CopyWebpackPlugin([
      {
        from: resolve(__dirname, 'node_modules/jsoneditor/dist/jsoneditor.min.css'),
        to: 'jsoneditor.css',
      }, {
        from: resolve(__dirname, 'node_modules/jsoneditor/dist/img/jsoneditor-icons.svg'),
        to: 'img/jsoneditor-icons.svg',
      },
      { from: resolve(__dirname, 'build/canvaskit/canvaskit.js') },
      { from: resolve(__dirname, 'build/canvaskit/canvaskit.wasm') },
      { from: resolve(__dirname, 'build/VERSION') },
      { from: resolve(__dirname, 'node_modules/@webcomponents/custom-elements/custom-elements.min.js') },
    ]),
  );

  config.node = {
    fs: 'empty',
  };

  return config;
};

export = configFactory;
