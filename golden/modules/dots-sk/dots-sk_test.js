import './index.js';
import { traces, commits } from './demo_data';
import {
  dotToCanvasX,
  dotToCanvasY,
  DOT_FILL_COLORS,
  DOT_FILL_COLORS_HIGHLIGHTED,
  DOT_RADIUS,
  DOT_STROKE_COLORS,
  TRACE_LINE_COLOR,
} from './constants';

describe('dots-sk', () => {
  let dotsSk;

  beforeEach(() => {
    dotsSk = document.createElement('dots-sk');
    dotsSk.value = traces;
    dotsSk.commits = commits;
    document.body.appendChild(dotsSk);
  });

  afterEach(() => {
    // Remove the stale instance under test.
    if (dotsSk) {
      document.body.removeChild(dotsSk);
      dotsSk = null;
    }
  });

  it('renders correctly', () => {
    expect(dotsSk._canvas.clientWidth).to.equal(210);
    expect(dotsSk._canvas.clientHeight).to.equal(40);
    expect(canvasToAscii(dotsSk)).to.equal(
        'hhgfeddeeddddccbbbaa\n' +
        '   bb-b-bbaa--aaaa  \n' +
        '      ccccbbbbbbaaaa');
  });

  it('highlights traces when hovering', async () => {
    // Hover over first trace. (X coordinate does not matter.)
    await hoverOverDot(dotsSk, 0, 0);
    expect(canvasToAscii(dotsSk)).to.equal(
        'HHGFEDDEEDDDDCCBBBAA\n' +
        '   bb-b-bbaa--aaaa  \n' +
        '      ccccbbbbbbaaaa');

    // Hover over second trace.
    await hoverOverDot(dotsSk,15, 1);
    expect(canvasToAscii(dotsSk)).to.equal(
        'hhgfeddeeddddccbbbaa\n' +
        '   BB-B-BBAA--AAAA  \n' +
        '      ccccbbbbbbaaaa');

    // Hover over third trace.
    await hoverOverDot(dotsSk,10, 2);
    expect(canvasToAscii(dotsSk)).to.equal(
        'hhgfeddeeddddccbbbaa\n' +
        '   bb-b-bbaa--aaaa  \n' +
        '      CCCCBBBBBBAAAA');
  });

  it('emits "hover" event when a trace is hovered', async () => {
    // Hover over first trace. (X coordinate does not matter.)
    let event = await hoverOverDotAndCatchHoverEvent(dotsSk, 0, 0);
    expect(event.detail).to.equal(',alpha=first-trace,beta=hello,gamma=world,');

    // Hover over second trace.
    event = await hoverOverDotAndCatchHoverEvent(dotsSk, 15, 1);
    expect(event.detail).to.equal(',alpha=second-trace,beta=foo,gamma=bar,');

    // Hover over third trace.
    event = await hoverOverDotAndCatchHoverEvent(dotsSk, 10, 2);
    expect(event.detail).to.equal(',alpha=third-trace,beta=baz,gamma=qux,');
  });

  it('emits "show-commits" event when a dot is clicked', async () => {
    // First trace, most recent commit.
    let event = await clickDotAndCatchShowCommitsEvent(dotsSk, 19, 0);
    expect(event.detail).to.deep.equal([commits[19]]);

    // First trace, middle-of-the-tile commit.
    event = await clickDotAndCatchShowCommitsEvent(dotsSk, 10, 0);
    expect(event.detail).to.deep.equal([commits[10]]);

    // First trace, oldest commit.
    event = await clickDotAndCatchShowCommitsEvent(dotsSk, 0, 0);
    expect(event.detail).to.deep.equal([commits[0]]);

    // Second trace, most recent commit with data
    event = await clickDotAndCatchShowCommitsEvent(dotsSk, 17, 1);
    expect(event.detail).to.deep.equal([commits[17]]);

    // Second trace, middle-of-the-tile dot preceded by two missing dots.
    event = await clickDotAndCatchShowCommitsEvent(dotsSk, 14, 1);
    expect(event.detail).to.deep.equal([commits[14], commits[13], commits[12]]);

    // Second trace, oldest commit with data preceded by three missing dots.
    event = await clickDotAndCatchShowCommitsEvent(dotsSk, 3, 1);
    expect(event.detail).to.deep.equal(
        [commits[3], commits[2], commits[1], commits[0]]);

    // Third trace, most recent commit.
    event = await clickDotAndCatchShowCommitsEvent(dotsSk, 19, 2);
    expect(event.detail).to.deep.equal([commits[19]]);

    // Third trace, middle-of-the-tile commit.
    event = await clickDotAndCatchShowCommitsEvent(dotsSk, 10, 2);
    expect(event.detail).to.deep.equal([commits[10]]);

    // Third trace, oldest commit.
    event = await clickDotAndCatchShowCommitsEvent(dotsSk, 6, 2);
    expect(event.detail).to.deep.equal([
      commits[6],
      commits[5],
      commits[4],
      commits[3],
      commits[2],
      commits[1],
      commits[0]
    ]);
  });
});

// Returns an ASCII-art representation of the canvas based on function
// dotToAscii.
const canvasToAscii = (dotsSk) => {
  const ascii = [];
  for (let y = 0; y < traces.traces.length; y++) {
    const trace = [];
    for (let x = 0; x < traces.tileSize; x++) {
      trace.push(dotToAscii(dotsSk, x, y));
    }
    ascii.push(trace.join(''));
  }
  return ascii.join('\n');
};

// Returns a character representing the dot at (x, y) in dotspace.
//   - A trace line is represented with '-'.
//   - A non-highlighted dot is represented with [a-g], where 'a' represents
//     the most recent commit.
//   - A highlighted dot is represented with [A-G].
//   - A blank position is represented with ' '.
const dotToAscii = (dotsSk, x,  y) => {
  const canvasX = dotToCanvasX(x);
  const canvasY = dotToCanvasY(y);

  // Sample a few pixels (north, east, south, west, center) from the bounding
  // box for the potential dot at (x, y). We'll use these to determine whether
  // there's a dot or a trace line at (x, y), what the color of the dot is,
  // whether or not it's highlighted, etc.
  const n = pixelAt(dotsSk, canvasX, canvasY - DOT_RADIUS);
  const e = pixelAt(dotsSk, canvasX + DOT_RADIUS, canvasY);
  const s = pixelAt(dotsSk, canvasX, canvasY + DOT_RADIUS);
  const w = pixelAt(dotsSk, canvasX - DOT_RADIUS, canvasY);
  const c = pixelAt(dotsSk, canvasX, canvasY);

  // Determines whether the sampled pixels match the given expected colors.
  const exactColorMatch =
      (en, ee, es, ew, ec) =>
          [n, e, s, w, c].toString() === [en, ee, es, ew, ec].toString();

  // Is it empty?
  const white = '#FFFFFF';
  if (exactColorMatch(white, white, white, white, white)) {
    return ' ';
  }

  // Is it a trace line?
  if (exactColorMatch(
      white, TRACE_LINE_COLOR, white, TRACE_LINE_COLOR, TRACE_LINE_COLOR)) {
    return '-';
  }

  // Iterate over all possible dot colors.
  for (let i = 0; i <= 7; i++) {
    // Is it a dot of the i-th color? Let's look at the pixels in the potential
    // circumference of the dot. Do they match the current color?
    // Note: we look for the closest match instead of an exact match due to
    // canvas anti-aliasing.
    if (closestColor(n, DOT_STROKE_COLORS) === DOT_STROKE_COLORS[i] &&
        closestColor(e, DOT_STROKE_COLORS) === DOT_STROKE_COLORS[i] &&
        closestColor(s, DOT_STROKE_COLORS) === DOT_STROKE_COLORS[i] &&
        closestColor(w, DOT_STROKE_COLORS) === DOT_STROKE_COLORS[i]) {

      // Is it a non-highlighted dot? (In other words, is it filled with the
      // corresponding non-highlighted color?)
      if (c === DOT_FILL_COLORS[i]) {
        return 'abcdefgh'[i];
      }

      // Is it a highlighted dot? (In other words, is it filled with the
      // corresponding highlighted color?)
      if (c === DOT_FILL_COLORS_HIGHLIGHTED[i]) {
        return 'ABCDEFGH'[i];
      }
    }
  }

  throw `unrecognized dot at (${x}, ${y})`
};

// Returns the color for the pixel at (x, y) in the canvas, represented as a hex
// string, e.g. "#AABBCC".
const pixelAt = (dotsSk, x, y) => {
  const pixel = dotsSk._ctx.getImageData(x, y, 1, 1).data;
  const r = pixel[0].toString(16).padStart(2, '0');
  const g = pixel[1].toString(16).padStart(2, '0');
  const b = pixel[2].toString(16).padStart(2, '0');
  return `#${r}${g}${b}`.toUpperCase();
};

// Finds the color in the haystack with the minimum Euclidean distance to the
// needle. This is necessary for pixels in the circumference of a dot due to
// canvas anti-aliasing. All colors are hex strings, e.g. "#AABBCC".
const closestColor = (needle, haystack) =>
  haystack
      .map(color => ({color: color, dist: euclideanDistanceSq(needle, color)}))
      .reduce((acc, cur) => (acc.dist < cur.dist) ? acc : cur)
      .color;

// Takes two colors represented as hex strings (e.g. "#AABBCC") and computes the
// squared Euclidean distance between them.
const euclideanDistanceSq = (color1, color2) => {
  const rgb1 = hexToRgb(color1), rgb2 = hexToRgb(color2);
  return Math.pow(rgb1[0] - rgb2[0], 2) +
         Math.pow(rgb1[1] - rgb2[1], 2) +
         Math.pow(rgb1[2] - rgb2[2], 2);
};

// Takes e.g. "#FF8000" and returns [256, 128, 0].
const hexToRgb = (hex) => {
  // Borrowed from https://stackoverflow.com/a/5624139.
  const res = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
  return [
    parseInt(res[1], 16),
    parseInt(res[2], 16),
    parseInt(res[3], 16),
  ];
};

// Returns a promise that will resolve when the given dots-sk instance emits the
// given event. The promise resolves to the caught event object.
const dotsSkEventPromise = (dotsSk, event) => {
  let resolve;
  const promise = new Promise((_resolve) => resolve = _resolve);
  const handler = (e) => {
    dotsSk.removeEventListener(event, handler);
    resolve(e);
  };
  dotsSk.addEventListener(event, handler);
  return promise;
};

// Simulate hovering over a dot.
const hoverOverDot = async (dotsSk, x, y) => {
  dotsSk._canvas.dispatchEvent(new MouseEvent('mousemove', {
    'clientX': dotsSk._canvas.getBoundingClientRect().left + dotToCanvasX(x),
    'clientY': dotsSk._canvas.getBoundingClientRect().top + dotToCanvasY(y),
  }));

  // Give mousemove event a chance to be processed. Necessary due to how
  // mousemove events are processed in batches by dots-sk every 40 ms.
  await new Promise((resolve) => setTimeout(resolve, 50));
};

// Simulate hovering over a dot, and return the "hover" CustomEvent emitted by
// the dots-sk instance.
const hoverOverDotAndCatchHoverEvent = async (dotsSk, x, y) => {
  const eventPromise = dotsSkEventPromise(dotsSk, 'hover');
  await hoverOverDot(dotsSk, x, y);
  return await eventPromise;
};

// Simulate clicking on a dot.
const clickDot = (dotsSk, x, y) => {
  dotsSk._canvas.dispatchEvent(new MouseEvent('click', {
    'clientX': dotsSk._canvas.getBoundingClientRect().left + dotToCanvasX(x),
    'clientY': dotsSk._canvas.getBoundingClientRect().top + dotToCanvasY(y),
  }));
};

// Simulate clicking on a dot, and return the "show-commits" CustomElement
// emitted by the dots-sk instance.
const clickDotAndCatchShowCommitsEvent = async (dotsSk, x, y) => {
  const eventPromise = dotsSkEventPromise(dotsSk, 'show-commits');
  clickDot(dotsSk, x, y);
  return await eventPromise;
};
