// / <reference path="../infra-sk/html-webpack-inject-attributes-plugin.d.ts" />

import { resolve } from 'path';
import webpack from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import HtmlWebpackInjectAttributesPlugin from 'html-webpack-inject-attributes-plugin';
import commonBuilder from '../infra-sk/pulito/webpack.common';

const configFactory: webpack.ConfigurationFactory = (_, args) => {
  // Don't minify the HTML since it contains Go template tags.
  const config = commonBuilder(__dirname, args.mode, /* neverMinifyHtml= */ true);

  config.output!.publicPath = '/dist/';

  config.plugins!.push(
    new HtmlWebpackInjectAttributesPlugin({
      nonce: '{% .Nonce %}',
    }),
  );

  config.resolve = config.resolve || {};

  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, 'node_modules')];


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
      { from: resolve(__dirname, 'build/canvaskit/canvaskit.d.ts') },
      { from: resolve(__dirname, 'build/canvaskit/canvaskit.wasm') },
      { from: resolve(__dirname, 'build/VERSION') },
    ]),
  );

  config.node = {
    fs: 'empty',
  };

  return config;
};

export = configFactory;
