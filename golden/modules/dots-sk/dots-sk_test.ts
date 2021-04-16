import './index';
import { DotsSk } from './dots-sk';
import { commits, traces } from './demo_data';
import {
  dotToCanvasX,
  dotToCanvasY,
  DOT_FILL_COLORS,
  DOT_FILL_COLORS_HIGHLIGHTED,
  DOT_RADIUS,
  DOT_STROKE_COLORS,
  MAX_UNIQUE_DIGESTS,
  TRACE_LINE_COLOR,
} from './constants';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { Commit } from '../rpc_types';

describe('dots-sk constants', () => {
  it('DOT_FILL_COLORS has the expected number of entries', () => {
    expect(DOT_FILL_COLORS).to.have.length(MAX_UNIQUE_DIGESTS);
  });

  it('DOT_FILL_COLORS_HIGHLIGHTED has the expected number of entries', () => {
    expect(DOT_FILL_COLORS_HIGHLIGHTED).to.have.length(MAX_UNIQUE_DIGESTS);
  });

  it('DOT_STROKE_COLORS has the expected number of entries', () => {
    expect(DOT_STROKE_COLORS).to.have.length(MAX_UNIQUE_DIGESTS);
  });
});

describe('dots-sk', () => {
  const newInstance = setUpElementUnderTest<DotsSk>('dots-sk');

  let dotsSk: DotsSk;
  let dotsSkCanvas: HTMLCanvasElement;
  let dotsSkCanvasCtx: CanvasRenderingContext2D;

  beforeEach(() => {
    dotsSk = newInstance((el) => {
      // All test cases use the same set of traces and commits.
      el.value = traces;
      el.commits = commits;
    });
    dotsSkCanvas = dotsSk.querySelector('canvas')!;
    dotsSkCanvasCtx = dotsSkCanvas.getContext('2d')!;
  });

  it('renders correctly', () => {
    expect(dotsSkCanvas.clientWidth).to.equal(210);
    expect(dotsSkCanvas.clientHeight).to.equal(40);
    // We specify the traces as an array and then join them instead of using a string literal
    // to avoid having invisible (but important to the test) trailing spaces.
    expect(canvasToAscii(dotsSkCanvasCtx)).to.equal([
      'iihgfddeeddddccbbbaa',
      '   bb-b-bbaa--aaaa  ',
      '      ccccbbbbbbaaaa',
    ].join('\n'));
  });

  it('highlights traces when hovering', async () => {
    // Hover over first trace. (X coordinate does not matter.)
    await hoverOverDot(dotsSkCanvas, 0, 0);
    expect(canvasToAscii(dotsSkCanvasCtx)).to.equal([
      'IIHGFDDEEDDDDCCBBBAA',
      '   bb-b-bbaa--aaaa  ',
      '      ccccbbbbbbaaaa',
    ].join('\n'));

    // Hover over second trace.
    await hoverOverDot(dotsSkCanvas, 15, 1);
    expect(canvasToAscii(dotsSkCanvasCtx)).to.equal([
      'iihgfddeeddddccbbbaa',
      '   BB-B-BBAA--AAAA  ',
      '      ccccbbbbbbaaaa',
    ].join('\n'));

    // Hover over third trace.
    await hoverOverDot(dotsSkCanvas, 10, 2);
    expect(canvasToAscii(dotsSkCanvasCtx)).to.equal([
      'iihgfddeeddddccbbbaa',
      '   bb-b-bbaa--aaaa  ',
      '      CCCCBBBBBBAAAA',
    ].join('\n'));
  });

  it('emits "hover" event when a trace is hovered', async () => {
    // Hover over first trace. (X coordinate does not matter.)
    let traceLabel = await hoverOverDotAndCatchHoverEvent(dotsSkCanvas, 0, 0);
    expect(traceLabel).to.equal(',alpha=first-trace,beta=hello,gamma=world,');

    // Hover over second trace.
    traceLabel = await hoverOverDotAndCatchHoverEvent(dotsSkCanvas, 15, 1);
    expect(traceLabel).to.equal(',alpha=second-trace,beta=foo,gamma=bar,');

    // Hover over third trace.
    traceLabel = await hoverOverDotAndCatchHoverEvent(dotsSkCanvas, 10, 2);
    expect(traceLabel).to.equal(',alpha=third-trace,beta=baz,gamma=qux,');
  });

  it('emits "showblamelist" event when a dot is clicked', async () => {
    // First trace, most recent commit.
    let dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 19, 0);
    expect(dotCommits).to.deep.equal([commits[19], commits[18]]);

    // First trace, middle-of-the-tile commit.
    dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 10, 0);
    expect(dotCommits).to.deep.equal([commits[10], commits[9]]);

    // First trace, oldest commit.
    dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 0, 0);
    expect(dotCommits).to.deep.equal([commits[0]]);

    // Second trace, most recent commit with data
    dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 17, 1);
    expect(dotCommits).to.deep.equal([commits[17], commits[16]]);

    // Second trace, middle-of-the-tile dot preceded by two missing dots.
    dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 14, 1);
    expect(dotCommits).to.deep.equal([commits[14], commits[13], commits[12], commits[11]]);

    // Second trace, oldest commit with data preceded by three missing dots.
    dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 3, 1);
    expect(dotCommits).to.deep.equal(
      [commits[3], commits[2], commits[1], commits[0]],
    );

    // Third trace, most recent commit.
    dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 19, 2);
    expect(dotCommits).to.deep.equal([commits[19], commits[18]]);

    // Third trace, middle-of-the-tile commit.
    dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 10, 2);
    expect(dotCommits).to.deep.equal([commits[10], commits[9]]);

    // Third trace, oldest commit.
    dotCommits = await clickDotAndCatchShowBlamelistEvent(dotsSkCanvas, 6, 2);
    expect(dotCommits).to.deep.equal([
      commits[6],
      commits[5],
      commits[4],
      commits[3],
      commits[2],
      commits[1],
      commits[0],
    ]);
  });
});

// Returns an ASCII-art representation of the canvas based on function
// dotToAscii.
function canvasToAscii(dotsSkCanvasCtx: CanvasRenderingContext2D): string {
  const ascii = [];
  for (let y = 0; y < traces.traces!.length; y++) {
    const trace = [];
    for (let x = 0; x < traces.tileSize; x++) {
      trace.push(dotToAscii(dotsSkCanvasCtx, x, y));
    }
    ascii.push(trace.join(''));
  }
  return ascii.join('\n');
}

// Returns a character representing the dot at (x, y) in dotspace.
//   - A trace line is represented with '-'.
//   - A non-highlighted dot is represented with a character in {'a', 'b', ...},
//     where 'a' represents the dot color for the most recent commit.
//   - A highlighted dot is represented with a character in {'A', 'B', ...}.
//   - A blank position is represented with ' '.
function dotToAscii(dotsSkCanvasCtx: CanvasRenderingContext2D, x: number, y: number): string {
  const canvasX = dotToCanvasX(x);
  const canvasY = dotToCanvasY(y);

  // Sample a few pixels (north, east, south, west, center) from the bounding
  // box for the potential dot at (x, y). We'll use these to determine whether
  // there's a dot or a trace line at (x, y), what the color of the dot is,
  // whether or not it's highlighted, etc.
  const n = pixelAt(dotsSkCanvasCtx, canvasX, canvasY - DOT_RADIUS);
  const e = pixelAt(dotsSkCanvasCtx, canvasX + DOT_RADIUS, canvasY);
  const s = pixelAt(dotsSkCanvasCtx, canvasX, canvasY + DOT_RADIUS);
  const w = pixelAt(dotsSkCanvasCtx, canvasX - DOT_RADIUS, canvasY);
  const c = pixelAt(dotsSkCanvasCtx, canvasX, canvasY);

  // Determines whether the sampled pixels match the given expected colors.
  const exactColorMatch = (en: string, ee: string, es: string, ew: string, ec: string) => {
    return [n, e, s, w, c].toString() === [en, ee, es, ew, ec].toString();
  };

  // Is it empty?
  const white = '#FFFFFF';
  if (exactColorMatch(white, white, white, white, white)) {
    return ' ';
  }

  // Is it a trace line?
  if (exactColorMatch(white, TRACE_LINE_COLOR, white, TRACE_LINE_COLOR, TRACE_LINE_COLOR)) {
    return '-';
  }

  // Iterate over all possible dot colors.
  for (let i = 0; i <= MAX_UNIQUE_DIGESTS; i++) {
    // Is it a dot of the i-th color? Let's look at the pixels in the potential
    // circumference of the dot. Do they match the current color?
    // Note: we look for the closest match instead of an exact match due to
    // canvas anti-aliasing.
    if (closestColor(n, DOT_STROKE_COLORS) === DOT_STROKE_COLORS[i]
        && closestColor(e, DOT_STROKE_COLORS) === DOT_STROKE_COLORS[i]
        && closestColor(s, DOT_STROKE_COLORS) === DOT_STROKE_COLORS[i]
        && closestColor(w, DOT_STROKE_COLORS) === DOT_STROKE_COLORS[i]) {
      // Is it a non-highlighted dot? (In other words, is it filled with the
      // corresponding non-highlighted color?)
      if (c === DOT_FILL_COLORS[i]) {
        return 'abcdefghijklmnopqrstuvwxyz'[i];
      }

      // Is it a highlighted dot? (In other words, is it filled with the
      // corresponding highlighted color?)
      if (c === DOT_FILL_COLORS_HIGHLIGHTED[i]) {
        return 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'[i];
      }
    }
  }

  throw `unrecognized dot at (${x}, ${y})`;
}

// Returns the color for the pixel at (x, y) in the canvas, represented as a hex
// string, e.g. "#AABBCC".
function pixelAt(dotsSkCanvasCtx: CanvasRenderingContext2D, x: number, y: number): string {
  const pixel = dotsSkCanvasCtx.getImageData(x, y, 1, 1).data;
  const r = pixel[0].toString(16).padStart(2, '0');
  const g = pixel[1].toString(16).padStart(2, '0');
  const b = pixel[2].toString(16).padStart(2, '0');
  return `#${r}${g}${b}`.toUpperCase();
}

// Finds the color in the haystack with the minimum Euclidean distance to the
// needle. This is necessary for pixels in the circumference of a dot due to
// canvas anti-aliasing. All colors are hex strings, e.g. "#AABBCC".
function closestColor(needle: string, haystack: string[]): string {
  return haystack
    .map((color) => ({ color: color, dist: euclideanDistanceSq(needle, color) }))
    .reduce((acc, cur) => ((acc.dist < cur.dist) ? acc : cur))
    .color;
}

// Takes two colors represented as hex strings (e.g. "#AABBCC") and computes the
// squared Euclidean distance between them.
function euclideanDistanceSq(color1: string, color2: string): number {
  const rgb1 = hexToRgb(color1);
  const rgb2 = hexToRgb(color2);
  return (rgb1[0] - rgb2[0]) ** 2 + (rgb1[1] - rgb2[1]) ** 2 + (rgb1[2] - rgb2[2]) ** 2;
}

// Takes e.g. "#FF8000" and returns [256, 128, 0].
function hexToRgb(hex: string): [number, number, number] {
  // Borrowed from https://stackoverflow.com/a/5624139.
  const res = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex)!;
  return [
    parseInt(res[1], 16),
    parseInt(res[2], 16),
    parseInt(res[3], 16),
  ];
}

// Simulate hovering over a dot.
async function hoverOverDot(dotsSkCanvas: HTMLCanvasElement, x: number, y: number) {
  dotsSkCanvas.dispatchEvent(new MouseEvent('mousemove', {
    clientX: dotsSkCanvas.getBoundingClientRect().left + dotToCanvasX(x),
    clientY: dotsSkCanvas.getBoundingClientRect().top + dotToCanvasY(y),
  }));

  // Give mousemove event a chance to be processed. Necessary due to how
  // mousemove events are processed in batches by dots-sk every 40 ms.
  await new Promise((resolve) => setTimeout(resolve, 50));
}

// Simulate hovering over a dot, and return the trace label in the "hover" event details.
async function hoverOverDotAndCatchHoverEvent(
    dotsSkCanvas: HTMLCanvasElement, x: number, y: number): Promise<string> {
  // const eventPromise = dotsSkEventPromise(dotsSk, 'hover');
  const event = eventPromise<CustomEvent<string>>('hover');
  await hoverOverDot(dotsSkCanvas, x, y);
  return (await event).detail;
}

// Simulate clicking on a dot.
function clickDot(dotsSkCanvas: HTMLCanvasElement, x: number, y: number) {
  dotsSkCanvas.dispatchEvent(new MouseEvent('click', {
    clientX: dotsSkCanvas.getBoundingClientRect().left + dotToCanvasX(x),
    clientY: dotsSkCanvas.getBoundingClientRect().top + dotToCanvasY(y),
  }));
}

// Simulate clicking on a dot, and return the list of commits in the "showblamelist" event details.
async function clickDotAndCatchShowBlamelistEvent(
    dotsSkCanvas: HTMLCanvasElement, x: number, y: number): Promise<Commit[]> {
  const event = eventPromise<CustomEvent<Commit[]>>('showblamelist');
  clickDot(dotsSkCanvas, x, y);
  return (await event).detail;
}
