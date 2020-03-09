// Constants that define how we go from dot space to canvas coordinates.
export const DOT_SCALE_X = 10;
export const DOT_SCALE_Y = 10;
export const DOT_OFFSET_X = 10;
export const DOT_OFFSET_Y = 10;

// Functions that go from dot space to canvas coordinates.
export const dotToCanvasX = (x) => x * DOT_SCALE_X + DOT_OFFSET_X;
export const dotToCanvasY = (y) => y * DOT_SCALE_Y + DOT_OFFSET_Y;

// Maximum number of unique digests to display. If the number of unique digests
// exceeds this, they will be grouped together with the last color.
// This corresponds to search.maxDistinctDigestsToPresent.
export const MAX_UNIQUE_DIGESTS = 9;

// Convention by the frontend to indicate there's no data for the given commit.
export const MISSING_DOT = -1;

// Constants that define what the traces look like. Colors are taken from the
// color blindness palette at http://mkweb.bcgsc.ca/colorblind.
export const TRACE_LINE_COLOR = '#999999';
export const STROKE_WIDTH = 2;  // Used for both the trace line and dots.
export const DOT_RADIUS = 3;

export const DOT_STROKE_COLORS = [
  '#000000',
  '#1B9E77',
  '#D95F02',
  '#7570B3',
  '#E7298A',
  '#66A61E',
  '#E6AB02',
  '#A6761D',
  '#999999',  // Used when the number of unique digests > MAX_UNIQUE_DIGESTS.
];
export const DOT_FILL_COLORS = [
  '#000000',
  '#FFFFFF',
  '#FFFFFF',
  '#FFFFFF',
  '#FFFFFF',
  '#FFFFFF',
  '#FFFFFF',
  '#FFFFFF',
  '#FFFFFF',  // Used when the number of unique digests > MAX_UNIQUE_DIGESTS.
];
export const DOT_FILL_COLORS_HIGHLIGHTED = [
  '#AAAAAA',
  '#1B9E77',
  '#D95F02',
  '#7570B3',
  '#E7298A',
  '#66A61E',
  '#E6AB02',
  '#A6761D',
  '#999999',  // Used when the number of unique digests > MAX_UNIQUE_DIGESTS.
];
