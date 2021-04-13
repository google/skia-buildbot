#!/bin/sh

allpackagejson=`find | grep "package\.json$" | grep -v html-template-minifier | grep -v jsdoc | grep -v node_modules | grep -v bower_components | grep -v .git | sort`
allpkgs=`jq -r '(.dependencies // {}) + (.devDependencies // {}) | keys | .[]' $allpackagejson | sort | uniq`
npm uninstall `jq -r '(.dependencies // {}) + (.devDependencies // {}) | keys | .[]' package.json`
git rm `find | grep "package\.json$" | grep -v ^./package.json$ | grep -v html-template-minifier | grep -v jsdoc | grep -v node_modules | grep -v bower_components | grep -v .git`
git rm `find | grep package-lock.json | grep -v ^./package-lock.json$ | grep -v html-template-minifier | grep -v jsdoc | grep -v node_modules | grep -v bower_components | grep -v .git`
npm install --save-dev $allpkgs
npm install --save-dev webpack@^4.0.0 @types/webpack@^4.0.0
npm install --save-dev webpack-dev-middleware@^3.0.0 @types/webpack-dev-middleware@^3.0.0
npm install --save-dev webpack-dev-server@^3.0.0 @types/webpack-dev-server@^3.0.0
npm install --save-dev webpack-cli@^3.0.0
npm install --save-dev mini-css-extract-plugin@^0.9.0 @types/mini-css-extract-plugin@^0.9.0
npm install --save-dev html-webpack-plugin@^4.0.0
npm install --save-dev copy-webpack-plugin@^5.0.0 @types/copy-webpack-plugin@^5.0.0
npm install --save-dev node-sass@^4.0.0 sass-loader@^7.0.0
npm install --save-dev postcss-loader@^2.0.0
npm install --save-dev autoprefixer@^9.0.0
npm install --save-dev html-loader@^0.5.5
npm install --save-dev cssnano@^3.0.0
npm install --save-dev karma@^4.0.0 @types/karma@^4.0.0
npm install --save-dev karma-webpack@^4.0.0
npm install --save-dev sinon@^7.0.0 @types/sinon@^7.0.0
npm install --save-dev d3-selection@^1.0.0
npm uninstall @bundled-es-modules/fetch-mock @types/fetch-mock
npm uninstall terser-webpack-plugin
npm install --save-dev @bazel/rollup@~2.0.0 @bazel/terser@~2.0.0 @bazel/typescript@~2.0.0
npm install --save-dev @rollup/plugin-commonjs@~15.0.0 @rollup/plugin-node-resolve@~9.0.0
npm install --save-dev terser@^5.0.0
