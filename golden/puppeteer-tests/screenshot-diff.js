const expect = require('chai').expect;
const fs = require('fs');
const path = require('path');
const PNG = require('pngjs').PNG;
const pixelmatch = require('pixelmatch');
const tmp = require('tmp');

// Directory relative to the caller's directory name where the screenshot
// goldens are located.
//
// E.g., if the caller test's path is /foo/bar/baz_test.js, we expect to find
// the reference screenshots at /foo/bar/<SCREENSHOT_GOLDENS_DIRNAME>.
const SCREENSHOT_GOLDENS_DIRNAME = 'screenshot_goldens';

exports.diffScreenshot = async (page, key, screenshotsDirAbsPath) => {
  // Validate key name.
  expect(key, 'invalid key format').to.match(/^[a-zA-Z0-9_-]+$/);

  // Get caller code information (absolute path and line number). Used to find
  // the path to the directory with golden PNGs and for better error reporting.
  const callerCodeInfo = getCallerCodeInfo();

  // If not provided, guess path to the screenshots directory based on the
  // caller filename.
  screenshotsDirAbsPath = screenshotsDirAbsPath
      ? screenshotsDirAbsPath
      : guessScreenshotsDirAbsPath(callerCodeInfo.fileName);
  expect(
      screenshotsDirAbsPath,
        'could not guess screenshotsDirAbsPath; please provide it explicitly')
      .to.not.be.empty;

  // Capture screenshot from page.
  const screenshotPng = PNG.sync.read(await page.screenshot());
  const width = page.viewport().width;
  const height = page.viewport().height;

  // Load golden screenshot.
  const goldenAbsPath = getGoldenAbsPath(key, screenshotsDirAbsPath);
  if (!fs.existsSync(goldenAbsPath)) {
    const errorMessage = 'Golden not found';
    const pathToReport =
        writeDiffReport(
            key,
            goldenAbsPath,
            screenshotPng,
            {
              callerFileName: callerCodeInfo.fileName,
              callerLineNumber: callerCodeInfo.lineNumber,
              errorMessage: errorMessage
            });
    expect.fail(makeErrorMessage(key, pathToReport, errorMessage));
  }
  const goldenPng = PNG.sync.read(fs.readFileSync(goldenAbsPath));

  // Fail early if the golden and screenshot differ in size.
  if (goldenPng.width !== width || goldenPng.height !== height) {
    const errorMessage = 'Golden and screenshot differ in size';
    const pathToReport =
        writeDiffReport(
            key,
            goldenAbsPath,
            screenshotPng,
            {
              callerFileName: callerCodeInfo.fileName,
              callerLineNumber: callerCodeInfo.lineNumber,
              goldenPng: goldenPng,
              errorMessage: errorMessage
            });
    expect.fail(
        makeErrorMessage(
            key, pathToReport, errorMessage));
  }

  // Compute the number of different pixels and render the diff image.
  const diffPng = new PNG({width, height});
  const pixelDelta = pixelmatch(goldenPng.data,
                                screenshotPng.data,
                                diffPng.data,
                                width,
                                height,
                        {threshold: 0});

  // Write the diff report if the two images aren't identical, and generate the
  // error message for the expect statement.
  if (pixelDelta !== 0) {
    const numPixels = pixelDelta === 1
        ? ' 1 pixel'
        : `${pixelDelta} pixels`;
    const errorMessage = `Golden and screenshot differ by ${numPixels}`;
    const pathToReport =
        writeDiffReport(
            key,
            goldenAbsPath,
            screenshotPng,
            {
              callerFileName: callerCodeInfo.fileName,
              callerLineNumber: callerCodeInfo.lineNumber,
              goldenPng: goldenPng,
              diffPng: diffPng,
              pixelDelta: pixelDelta,
              errorMessage: errorMessage
            });
    expect.fail(makeErrorMessage(key, pathToReport, errorMessage));
  }
};

// Returns the absolute path to the JS file and line number calling this module.
function getCallerCodeInfo() {
  // Get a stack trace using the V8 stack trace API. See
  // https://v8.dev/docs/stack-trace-api#customizing-stack-traces for details.
  let prepareStackTrace = Error.prepareStackTrace; // Save a backup.
  Error.prepareStackTrace = (_, stack) => stack;
  const stack = new Error().stack;
  Error.prepareStackTrace = prepareStackTrace;  // Restore from backup.

  // Get the file name of the top callsite, e.g. this file.
  const currentFileName = stack[0].getFileName();

  // Iterate over stack from top to bottom. Stop at the first frame outside of
  // this file.
  for(let i = 0; i < stack.length; i++) {
    const fileName = stack[i].getFileName();
    if (fileName !== currentFileName) {
      return {fileName: fileName, lineNumber: stack[i].getLineNumber()};
    }
  }

  // This should never happen.
  expect.fail(`could not find a stack frame outside of ${currentFileName}`);
}

function makeErrorMessage(key, pathToReport, message) {
  return `[${key}] ${message}. See file://${pathToReport}.`;
}

// Guess the absolute path to the directory with the screenshot goldens. Takes
// the absolute path to the code calling this module as a parameter, e.g.
// "/foo/bar/baz_test.js".
function guessScreenshotsDirAbsPath(callerAbsPath) {
  // If the caller doesn't appear to be a test, then function diffScreenshot is
  // being used in an atypical way. Abort attempt at guessing.
  if (!callerAbsPath.endsWith('_test.js')) {
    return null;
  }

  // E.g. /foo/bar/<SCREENSHOT_GOLDENS_DIRNAME>.
  return path.join(path.dirname(callerAbsPath), SCREENSHOT_GOLDENS_DIRNAME);
}

function getGoldenAbsPath(key, screenshotsDirAbsPath) {
  return path.join(screenshotsDirAbsPath, `${key}.png`);
}

function writeDiffReport(
    key, goldenAbsPath, screenshotPng, options) {
  const callerFileName = options.callerFileName;
  const callerLineNumber = options.callerLineNumber;
  const goldenPng = options.goldenPng;
  const diffPng = options.diffPng;
  const pixelDelta = options.pixelDelta;
  const errorMessage = options.errorMessage;

  // Create a new directory under /tmp to save the report and PNG files.
  const dir = tmp.dirSync({prefix: `screenshot-diff_${key}-`}).name;

  // Paths to files to be written.
  const reportFilePath = path.join(dir, 'diff.html');
  const screenshotFilePath = path.join(dir, 'screenshot.png');
  const goldenFilePath = path.join(dir, 'golden.png');
  const diffFilePath = path.join(dir, 'diff.png');

  // Screenshot approval command.
  const approvalCommand =
      `mkdir -p ${path.dirname(goldenAbsPath)} && ` +
      `cp ${screenshotFilePath} ${goldenAbsPath}`;

  // Save PNG files.
  fs.writeFileSync(screenshotFilePath, PNG.sync.write(screenshotPng));
  if (goldenPng) fs.writeFileSync(goldenFilePath, PNG.sync.write(goldenPng));
  if (diffPng) fs.writeFileSync(diffFilePath, PNG.sync.write(diffPng));

  // Test and results summary table.
  const summary = new Map([['Key', key]]);
  if (callerFileName && callerLineNumber) {
    summary.set("Called at", `${callerFileName}:${callerLineNumber}`);
  }
  if (errorMessage) {
    summary.set('Error message', errorMessage);
  }
  const imgSize = png => `${png.width} x ${png.height} px ` +
                         `(${png.width * png.height} pixels)`;
  summary.set('Screenshot size', imgSize(screenshotPng));
  if (goldenPng) {
    summary.set('Golden size', imgSize(goldenPng));
  }
  if (pixelDelta !== undefined) {
    summary.set('# different pixels', pixelDelta);
    summary.set(
        '% different pixels',
        100 * pixelDelta / (screenshotPng.width * screenshotPng.height));
  }

  const title = `Screenshot Diff Test Results: ${key}`;

  const html = `
<!DOCTYPE html>
<html>
<head>
<title>${title}</title>
<style>
  body {
    margin: 0 1em 1em;
    padding: 0;
    font-family: 'Roboto', 'Noto', sans-serif;
    -webkit-font-smoothing: antialiased;
    color: #222222;
    font-size: 16px;
    line-height: 24px;
  }
  
  h1 {
    font-size: 40px;
    line-height: 48px;
    font-weight: normal;
  }
  
  h2 {
    font-size: 28px;
    line-height: 36px;
    font-weight: normal;
  }
  
  table.summary td:first-child {
    font-weight: bold;
    padding-right: 1em;
    text-align: right;
  }
  
  div.tabs {
    display: flex;
    align-items: center;
  }

  div.tabs h2 {
    margin-right: 1em;
    cursor: pointer;
  }

  div.tabs h2:hover,
  div.tabs h2.selected {
    text-decoration: underline;
  }

  div.tabs small {
    margin-left: auto;
  }

  div.tab-contents section {
    display: none;
  }

  div.tab-contents section.selected {
    display: block;
  }
  
  div.tab-contents img {
    border: 1px solid #f5f5f5;
  }
</style>
<script>
  function toggleImage(id) {
    // Mark tab as selected.
    document.querySelector('div.tabs h2.selected').classList.remove('selected');
    document.querySelector(\`div.tabs h2[data-target=\${id}]\`).classList.add('selected');

    // Show the tab's contents.
    document.querySelector('div.tab-contents section.selected').classList.remove('selected');
    document.querySelector(\`div.tab-contents section#\${id}\`).classList.add('selected');
  }

  function advanceImage(delta) {
    const ids = Array.from(document.querySelectorAll('div.tab-contents section')).map(el => el.id);
    const selectedId = document.querySelector('div.tab-contents section.selected').id;
    const currentIndex = ids.indexOf(selectedId);
    const newIndex = ((currentIndex + delta % ids.length) + ids.length) % ids.length; // a mod b.
    toggleImage(ids[newIndex]);
  }

  document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('div.tabs h2').forEach(el => {
      el.addEventListener('click', (ev) => {
        toggleImage(ev.target.dataset.target);
      })
    });
  });

  document.addEventListener('keydown', function (event) {
    if (event.key === 'ArrowRight' || event.key === 'j') {
      advanceImage(1);
      event.preventDefault();
    }
    if (event.key === 'ArrowLeft' || event.key === 'k') {
      advanceImage(-1);
      event.preventDefault();
    }
  });
</script>
</head>
<body>
<h1>${title}</h1>

<table class=summary>
${Array.from(summary)
       .map(kv => `<tr><td>${kv[0]}</td><td>${kv[1]}</td></tr>`)
       .join('\n')}
</table>

<div class=tabs>
  <h2 data-target=screenshot class=selected>Screenshot</h2>
  ${!goldenPng ? '' : '<h2 data-target=golden>Golden</h2>'}
  ${!diffPng ? '' : '<h2 data-target=diff>Diff</h2>'}
  ${goldenPng || diffPng ? `
  <small>
    <strong>Tip:</strong> Use the arrow keys (or j/k) to switch between tabs.
  </small>
  ` : ''}
</div>

<div class=tab-contents>
  <section id=screenshot class=selected>
    <img src=screenshot.png alt=screenshot/>  
  </section>
  
  ${goldenPng ? `
  <section id=golden>
    <img src=golden.png alt=golden/>
  </section>
  ` : ''}
 
  ${diffPng ? `
  <section id=diff>
    <img src=diff.png alt=diff/>
  </section>
  ` : ''}
</div>
    
<h2>Approve changes</h2>
<p>
  Run the following command to set the screenshot as the new golden:
</p>
<textarea readonly rows=5 cols=80 onclick=this.select()>${approvalCommand}</textarea>
</body>
</html>`.trimStart();

  fs.writeFileSync(reportFilePath, html);
  return reportFilePath;
}