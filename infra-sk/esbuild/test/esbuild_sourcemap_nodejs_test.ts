/**
 * Test the sourcemap generating behaviors of the esbuild_dev_bundle and esbuild_prod_bundle rules.
 *
 * Even though esbuild seems to behave as expected, the "sourcemap" attribute of esbuild Bazel rule
 * provided by the rules_nodejs does not seem to support not generating any sourcemaps at all. We
 * override this behavior by specifying extra arguments to the esbuild binary via the esbuild
 * rule's "args" argument, see //infra-sk/esbuild/esbuild.bzl for details. Given the hacky nature
 * of this workaround, it seems prudent to test it.
 *
 * To learn more about sourcemaps, see:
 *  - https://sourcemaps.info/spec.html
 *  - https://developers.google.com/web/updates/2013/06/sourceMappingURL-and-sourceURL-syntax-changed
 */

import fs from 'fs';
import path from 'path';

import { expect } from 'chai';

// Inspired in
// https://github.com/bazelbuild/rules_nodejs/blob/bca4dbeba5bf3be023aea602ea3eae2dee2ce10f/packages/esbuild/test/sourcemap/bundle_test.js#L4.
const runfilesHelper = require(process.env.BAZEL_NODE_RUNFILES_HELPER);
const locationBase = 'skia_infra/infra-sk/esbuild/test';

function doesRunfileExist(filename: string): boolean {
  return fs.existsSync(path.join(locationBase, filename));
}

function readBundle(filename: string): string {
  return fs.readFileSync(runfilesHelper.resolve(path.join(locationBase, filename)), 'utf8');
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
