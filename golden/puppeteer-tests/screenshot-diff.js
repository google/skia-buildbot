const fs = require('fs');
const PNG = require('pngjs').PNG;
const pixelmatch = require('pixelmatch');

exports.diffScreenshot = async (page, goldenPngImagePath) => {
  const width = page.viewport().width;
  const height = page.viewport().height;

  const golden = PNG.sync.read(fs.readFileSync(goldenPngImagePath));
  const screenshot = PNG.sync.read(await page.screenshot());
  const diff = new PNG({width, height});

  const delta = pixelmatch(golden.data,
                           screenshot.data,
                           diff.data,
                           width,
                           height,
                   {threshold: 0});

  console.log('Number of different pixels: ' + delta);

  const actualPath = goldenPngImagePath + '.actual.png';
  fs.writeFileSync(actualPath, PNG.sync.write(screenshot));
  console.log('actual written to: ' + actualPath);

  const diffPath = goldenPngImagePath + '.diff.png';
  fs.writeFileSync(diffPath, PNG.sync.write(diff));
  console.log('diff written to: ' + diffPath);
};