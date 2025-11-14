/**
 * Test the sourcemap generating behaviors of the esbuild_dev_bundle and esbuild_prod_bundle rules.
 *
 * To learn more about sourcemaps, see:
 *  - https://sourcemaps.info/spec.html
 *  - https://developers.google.com/web/updates/2013/06/sourceMappingURL-and-sourceURL-syntax-changed
 */

import fs from 'fs';
import path from 'path';

import { expect } from 'chai';

// See https://docs.aspect.build/rulesets/aspect_rules_js/docs/js_binary#js_binary.
const runfilesDir = process.env.JS_BINARY__RUNFILES!;

const locationBase = '_main/infra-sk/esbuild/test';

function doesRunfileExist(filename: string): boolean {
  return fs.existsSync(path.join(runfilesDir, locationBase, filename));
}

function readBundle(filename: string): string {
  return fs.readFileSync(path.join(runfilesDir, locationBase, filename), 'utf-8');
}

describe('esbuild sourcemaps', () => {
  it('produces development bundles with inline sourcemaps', () => {
    const bundleContents = readBundle('dev_bundle.js');
    expect(bundleContents).to.contain('//# sourceMappingURL=data:application/json;base64');
    expect(bundleContents).not.to.contain('//@ sourceMappingURL');
    expect(bundleContents).not.to.contain('//# sourceURL');
    expect(bundleContents).not.to.contain('//@ sourceURL');
    expect(doesRunfileExist('dev_bundle.js.map')).to.be.false;
  });

  it('produces production bundles without sourcemaps of any kind', () => {
    const bundleContents = readBundle('prod_bundle.js');
    expect(bundleContents).not.to.contain('//# sourceMappingURL');
    expect(bundleContents).not.to.contain('//@ sourceMappingURL');
    expect(bundleContents).not.to.contain('//# sourceURL');
    expect(bundleContents).not.to.contain('//@ sourceURL');
    expect(doesRunfileExist('prod_bundle.js.map')).to.be.false;
  });
});
