import * as fs from 'fs';
import * as path from 'path';

/**
 * Returns the output directory where tests should e.g. save screenshots.
 * Screenshots saved in this directory will be uploaded to Gold.
 */
export const outputDir = () => {
  // Screenshots will be saved as test undeclared outputs, which will be found at
  // at //_bazel_testlogs/path/to/my/puppeteer_test/test.outputs/outputs.zip. This is true when
  // running on RBE as well (e.g. "bazel test --config=remote").
  //
  // See the following link for more:
  // https://docs.bazel.build/versions/master/test-encyclopedia.html#test-interaction-with-the-filesystem.
  const undeclaredOutputsDir = process.env.TEST_UNDECLARED_OUTPUTS_DIR;
  if (!undeclaredOutputsDir) {
    throw new Error('required environment variable TEST_UNDECLARED_OUTPUTS_DIR is unset');
  }
  const outputDir = path.join(undeclaredOutputsDir, 'puppeteer-test-screenshots');
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir);
  }
  return outputDir;
};
