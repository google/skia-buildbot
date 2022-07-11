/**
 * @module modules/plot-simple-sk
 * @description <h2><code>plot-simple-sk</code></h2>
 *
 *  A custom element for plotting x,y graphs.
 *
 *  The canvas is broken into two areas, the summary and the details. The
 *  summary is always SUMMARY_HEIGHT pixels high. Also note that we use
 *  window.devicePixelRatio to decide the actual number of pixels to use, and
 *  then use CSS transform to squash the canvas back down to the desired size.
 *
 *    +----------------------------------------------------+
 *    |                                                    |
 *    |                   MARGIN                           |
 *    |                                                    |
 *    |   +--------------------------------------------+   |
 *    |   |           Summary                          |   |
 *    |   |                                            |   |
 *    |   +--------------------------------------------+   |
 *    |                                                    |
 *    |                   MARGIN                           |
 *    |                                                    |
 *    |   +--------------------------------------------+   |
 *    |   |          Details                           |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   +--------------------------------------------+   |
 *    |                                                    |
 *    |                   MARGIN                           |
 *    |                                                    |
 *    +----------------------------------------------------+
 *
 * To keep rendering quick the traces will be written into Path2D objects to be
 * used for quick rendering.
 *
 * We also use a k-d Tree for quick lookup for clicking and mouse movement over
 * the traces.
 *
 * There are actually two canvas's in play, the trace canvas is below the
 * overlay canvas. The trace canvas contains all the traces in the summary and
 * details along with their axes. The overlay canvas contains everything that
 * changes quickly, such as the crosshairs, the x-bar, etc.
 *
 * This element knows about elements-sk/themes and uses those CSS variables if
 * present.
 *
 * Listens for "theme-chooser-toggle" event on the document and redraws with
 * updated computed style colors.
 *
 * @evt trace_selected - Event produced when the user clicks on a line. The
 *     e.detail contains the id of the line and the index of the point in the
 *     line closest to the mouse, and the [x, y] value of the point in 'pt'.
 *
 *     <pre>
 *     {
 *        x: x,
 *        y: y,
 *        name: name,
 *      }
 *     </pre>
 *
 * @evt trace_focused - Event produced when the user moves the mouse close to a
 *     line. The e.detail contains the id of the line and the index of the point
 *     in the line closest to the mouse.
 *
 *     <pre>
 *     {
 *        x: x,
 *        y: y,
 *        name: name,
 *      }
 *     </pre>
 *
 * @evt zoom - Event produced when the user has zoomed into a region by
 *      dragging. The detail is of the form:
 *
 *      {
 *        xBegin: new Date(),
 *        xEnd: new Date(),
 *      }
 *
 * @attr width - The width of the element in px.
 *
 * @attr height - The height of the element in px.
 *
 * @attr summary {Boolean} - If present then display the summary bar.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import * as d3Scale from 'd3-scale';
import * as d3Array from 'd3-array';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { KDTree, KDPoint } from './kd';
import { ticks } from './ticks';

//  Prefix for trace ids that are not real traces, such as special_zero. Special
//  traces never receive focus and can't be clicked on.
const SPECIAL = 'special';

export const MISSING_DATA_SENTINEL = 1e32;

const NUM_Y_TICKS = 4;

const HOVER_COLOR = '#8887'; // Note the alpha value.

const ZOOM_RECT_COLOR = '#0007'; // Note the alpha value.

const SUMMARY_LINE_WIDTH = 1; // px

const DETAIL_LINE_WIDTH = 1; // px

const AXIS_LINE_WIDTH = 1; // px

const MIN_MOUSE_MOVE_FOR_ZOOM = 5; // px

/**
 * @constant {Array} - Colors used for traces.
 */
const COLORS = [
  '#000000',
  '#1B9E77',
  '#D95F02',
  '#7570B3',
  '#E7298A',
  '#66A61E',
  '#E6AB02',
  '#A6761D',
  '#666666',
];

// Contains linear scales to convert from source coordinates into
// device/destination coordinates.
export interface Range {
  x: d3Scale.ScaleLinear<number, number>;
  y: d3Scale.ScaleLinear<number, number>;
}

// A trace is drawn as a set of lines overdrawn with dots at each measurement.
interface TracePaths {
  linePath: Path2D | null;
  dotsPath: Path2D | null;
}

/** @class Builds the Path2D objects that describe the trace and the dots for a given
 *   set of scales.
 */
class PathBuilder {
  // TODO(jcgregorio) Change to TracePaths.
  private linePath: Path2D;

  private dotsPath: Path2D;

  // TODO(jcgregorio) Change to Range.
  private xRange: d3Scale.ScaleLinear<number, number>;

  private yRange: d3Scale.ScaleLinear<number, number>;

  private radius: number;

  constructor(
    xRange: d3Scale.ScaleLinear<number, number>,
    yRange: d3Scale.ScaleLinear<number, number>,
    radius: number,
  ) {
    this.xRange = xRange;
    this.yRange = yRange;
    this.radius = radius;
    this.linePath = new Path2D();
    this.dotsPath = new Path2D();
  }

  /**
   * Add a point to plot to the path.
   *
   * @param {Number} x - X coordinate in source coordinates.
   * @param {Number} y - Y coordinate in source coordinates.
   */
  add(x: number, y: number) {
    // Convert source coord into canvas coords.
    const cx = this.xRange(x);
    const cy = this.yRange(y);

    if (x === 0) {
      this.linePath.moveTo(cx, cy);
    } else {
      this.linePath.lineTo(cx, cy);
    }
    this.dotsPath.moveTo(cx + this.radius, cy);
    this.dotsPath.arc(cx, cy, this.radius, 0, 2 * Math.PI);
  }

  /**
   * Returns the Arrays of Path2D objects that represent all the traces.
   *
   * @returns {Object}
   */
  paths(): TracePaths {
    return {
      linePath: this.linePath,
      dotsPath: this.dotsPath,
    };
  }
}

interface Point {
  x: number;
  y: number;
}

const invalidPoint: Point = {
  x: Number.MIN_SAFE_INTEGER,
  y: Number.MIN_SAFE_INTEGER,
};

const pointIsValid = (p: Point): boolean => p.x !== invalidPoint.x;

interface Rect extends Point {
  width: number;
  height: number;
}

/**
 * Convert rect in domain units into canvas coordinates using the given range.
 *
 * Presumes the rect was previously gotten from rectFromRangeInvert, so we don't
 * need to flip top and bottom.
 */
export const rectFromRange = (range: Range, rect: Rect): Rect => {
  const cleft = range.x(rect.x);
  const ctop = range.y(rect.y);
  const cright = range.x(rect.x + rect.width);
  const cbottom = range.y(rect.y + rect.height);
  return {
    x: cleft,
    y: ctop,
    width: cright - cleft,
    height: cbottom - ctop,
  };
};

/**
 * Convert rect in canvas units into domain units given the given range.
 *
 * Presumes this comes from a dragged out mouse region, which could be backwards
 * and/or upside down, so corrections are done to the corners.
 */
export const rectFromRangeInvert = (range: Range, rect: Rect): Rect => {
  let left = rect.x;
  let top = rect.y;
  let right = rect.x + rect.width;
  let bottom = rect.y + rect.height;
  if (right < left) {
    [left, right] = [right, left];
  }
  // We do this backwards since range.y then does a second direction reversal,
  // i.e. Canvas y-axis is in the opposite direction of the y-axis we use in
  // the data units.
  if (top < bottom) {
    [bottom, top] = [top, bottom];
  }

  return {
    x: range.x.invert(left),
    y: range.y.invert(top),
    width: range.x.invert(right) - range.x.invert(left),
    height: range.y.invert(bottom) - range.y.invert(top),
  };
};

const defaultRect: Rect = {
  x: 0,
  y: 0,
  width: 0,
  height: 0,
};

interface SearchPoint extends Point {
  // Source coordinates.
  sx: number;
  sy: number;
  name: string;
}

/**
 * @class Builds a kdTree for searcing for nearest points to the mouse.
 */
class SearchBuilder {
  // TODO(jcgregorio) Change to Range.
  private xRange: d3Scale.ScaleLinear<number, number>;

  private yRange: d3Scale.ScaleLinear<number, number>;

  private points: SearchPoint[];

  constructor(
    xRange: d3Scale.ScaleLinear<number, number>,
    yRange: d3Scale.ScaleLinear<number, number>,
  ) {
    this.xRange = xRange;
    this.yRange = yRange;
    this.points = [];
  }

  /**
   * Add a point to the kdTree.
   *
   * Note that add() stores the x and y coords as 'sx' and 'sy' in the KDTree nodes,
   * and the canvas coords, which are computed from sx and sy are stored as 'x'
   * and 'y' in the KDTree nodes.
   *
   * @param {Number} x - X coordinate in source coordinates.
   * @param {Number} y - Y coordinate in source coordinates.
   * @param {String} name - The trace name.
   */
  add(x: number, y: number, name: string) {
    if (name.startsWith(SPECIAL)) {
      return;
    }

    // Convert source coord into canvas coords.
    const cx = this.xRange(x);
    const cy = this.yRange(y);

    this.points.push({
      x: cx,
      y: cy,
      sx: x,
      sy: y,
      name,
    });
  }

  /**
   * Returns a kdTree that contains all the points being plotted.
   *
   * @returns {KDTree}
   */
  kdTree() {
    const distance = (a: KDPoint, b: KDPoint) => {
      const dx = a.x - b.x;
      const dy = a.y - b.y;
      return dx * dx + dy * dy;
    };

    return new KDTree(this.points, distance, ['x', 'y']);
  }
}

// Returns true if pt is in rect.
function inRect(pt: Point, rect: Rect): boolean {
  return (
    pt.x >= rect.x
    && pt.x < rect.x + rect.width
    && pt.y >= rect.y
    && pt.y < rect.y + rect.height
  );
}

// Restricts pt to rect.
function clampToRect(pt: Point, rect: Rect) {
  if (pt.x < rect.x) {
    pt.x = rect.x;
  } else if (pt.x > rect.x + rect.width) {
    pt.x = rect.x + rect.width;
  }
  if (pt.y < rect.y) {
    pt.y = rect.y;
  } else if (pt.y > rect.y + rect.height) {
    pt.y = rect.y + rect.height;
  }
}

// Clip the given Canvas2D context to the given rect.
function clipToRect(ctx: CanvasRenderingContext2D, rect: Rect) {
  ctx.beginPath();
  ctx.rect(rect.x, rect.y, rect.width, rect.height);
  ctx.clip();
}

// All the data for a single trace.
interface LineData {
  name: string;
  values: number[];
  color: string;
  detail: TracePaths;
  summary: TracePaths;
}

interface MousePosition {
  clientX: number;
  clientY: number;
}

interface MouseMoveRaw extends MousePosition {
  // Is the shift key being pressed as the mouse moves.
  shiftKey: boolean;
}

interface HoverPoint extends Point {
  // The trace id.
  name: string;
}

interface Label extends Point {
  // The text value of the label.
  text: string;
}

interface CrosshairPoint extends Point {
  shift: boolean;
}

// Common information for both the Summary and Detail display areas.
interface Area {
  rect: Rect;
  axis: {
    path: Path2D;
    labels: Label[];
  };
  range: Range;
}

type SummaryArea = Area

interface DetailArea extends Area {
  yaxis: {
    path: Path2D;
    labels: Label[];
  };
}

// Describes the zoom in terms of x-axis source values.
type ZoomRange = [number, number] | null;

// Used for both the trace_selected and trace_focused events.
export interface PlotSimpleSkTraceEventDetails {
  x: number;
  y: number;

  // The trace id.
  name: string;
}

export interface PlotSimpleSkZoomEventDetails {
  xBegin: Date;
  xEnd: Date;
}

/**
 * The type of zoom being done, or 'no-zoom' if no zoom is currently being done.
 */
export type ZoomDragType = 'no-zoom' | 'details' | 'summary'

export class PlotSimpleSk extends ElementSk {
  /** The location of the XBar. See the xbar property. */
  private _xbar: number = -1;

  /** If true then draw dots on the traces. */
  private _dots: boolean = true;

  /** The locations of the background bands. See bands property. */
  private _bands: number[] = [];

  /** A map of trace names to 'true' of traces that are highlighted. */
  private highlighted: { [key: string]: boolean } = {};

  /**
   *  The data we are plotting.
   *  An array of objects of this form:
   *
   *   {
   *     name: key,
   *     values: [1.0, 1.1, 0.9, ...],
   *     detail: {
   *       linePath: Path2D,
   *       dotsPath: Path2D,
   *     },
   *     summary: {
   *       linePath: Path2D,
   *       dotsPath: Path2D,
   *     },
   *   }
   */
  private lineData: LineData[] = [];

  /** An array of Date()'s the same length as the values in lineData. */
  private labels: Date[] = [];

  /**
   * The current zoom, either null or an array of two values in source x
   * coordinates, e.g. [1, 12].
   */
  private _zoom: ZoomRange = null;

  /** The source coordinate where a zoom started. */
  private zoomBegin: number = 0;

  /** The zoom rectangle on the details region. Stored in destination units. */
  private zoomRect: Rect = defaultRect;

  /**
   * detailsZoomRangesStack and inactiveZoomRangesStack work together to manage
   * a stack of zoom levels. As new zoom ranges are dragged out they are pushed
   * onto detailsZoomRangesStack, and as the mouse wheel is turned we push/pop
   * zoom ranges between detailsZoomRangesStack and inactiveZoomRangesStack.
   * Scroll up means to zoom in, so we pop a rect off the inactive stack and
   * push it on the active stack. Scrolling down means to zoom out, so we do the
   * reverse, pop of the active stack and push it onto the inactive stack.
   *
   * When plotting we only look at the top of the active stack, i.e. the end of
   * detailsZoomRangesStack.
   *
   * detailsZoomRangesStack is a stack of zoom ranges, each zoom range in the
   * stack represents a smaller area than the zoom range below it on the stack.
   * Zoom ranges are stored in domain units.
   */
  private detailsZoomRangesStack: Rect[] = [];

  /**
   * As we use the mouse wheel to move through zooms we store the ones we've
   * popped off of detailsZoomRangesStack here.
   *
   * Zoom ranges are stored in domain units.
   */
  private inactiveDetailsZoomRangesStack: Rect[] = [];

  /**
   * True if we are currently drag zooming, i.e. the mouse is pressed and moving
   * over the summary.
   */
  private inZoomDrag: ZoomDragType = 'no-zoom';

  /** The Canvas 2D context of the traces canvas. */
  private ctx: CanvasRenderingContext2D | null = null;

  /** The Canvas 2D context of the overlay canvas. */
  private overlayCtx: CanvasRenderingContext2D | null = null;

  /** The window.devicePixelRatio. */
  private scale: number = 1.0;

  /**
   * A copy of the clientX, clientY, and shiftKey values of mousemove events,
   * or null if a mousemove event hasn't occurred since the last time it was
   * processed.
   */
  private mouseMoveRaw: MouseMoveRaw | null = null;

  /**
   * A kdTree for all the points being displayed, in source coordinates. Is
   * null if no traces are being displayed.
   */
  private pointSearch: KDTree<SearchPoint> | null = null;

  /**
   * The closest trace point to the mouse. May be {} if no traces are
   * displayed or the mouse hasn't moved over the canvas yet. Has the form:
   *   {
   *     x: x,
   *     y: y,
   *     name: String, // name of trace
   *   }
   */
  private hoverPt: HoverPoint = {
    x: -1,
    y: -1,
    name: '',
  };

  /**
   * The location of the crosshair in canvas coordinates. Of the form:
   *   {
   *     x: x,
   *     y: y,
   *     shift: Boolean,
   *   }
   *
   * The value of shift is true of the shift key is being pressed while the
   * mouse moves.
   * }
   */
  private crosshair: CrosshairPoint = {
    x: -1,
    y: -1,
    shift: false,
  };

  /** All the info we need about the summary area. */
  private summaryArea: SummaryArea = {
    rect: {
      x: 0,
      y: 0,
      width: 0,
      height: 0,
    },
    axis: {
      path: new Path2D(), // Path2D.
      labels: [], // The labels and locations to draw them. {x, y, text}.
    },
    range: {
      x: d3Scale.scaleLinear(),
      y: d3Scale.scaleLinear(),
    },
  };

  /** All the info we need about the details area. */
  private detailArea: DetailArea = {
    rect: {
      x: 0,
      y: 0,
      width: 0,
      height: 0,
    },
    axis: {
      path: new Path2D(), // Path2D.
      labels: [], // The labels and locations to draw them. {x, y, text}.
    },
    yaxis: {
      path: new Path2D(),
      labels: [], // The labels and locations to draw them. {x, y, text}.
    },
    range: {
      x: d3Scale.scaleLinear(),
      y: d3Scale.scaleLinear(),
    },
  };

  /**
   * A task to rebuild the k-d search tree used for finding the closest point
   * to the mouse. The actual value is a window.setTimer timerId or zero if no
   * task is scheduled.
   *
   * See https://jakearchibald.com/2015/tasks-microtasks-queues-and-schedules/
   * for details on tasks vs microtasks.
   */
  private recalcSearchTask: number = 0;

  /**
   * A task to do the actual re-draw work of a zoom. The actual value is a
   * window.setTimer timerId or zero if no task is scheduled.
   *
   * See https://jakearchibald.com/2015/tasks-microtasks-queues-and-schedules/
   * for details on tasks vs microtasks.
   */
  private zoomTask: number = 0;

  /**
   * A formatter that prints numbers nicely, such as adding commas. Used when
   * display the hover text.
   */
  private numberFormatter: Intl.NumberFormat = new Intl.NumberFormat();

  private SUMMARY_HEIGHT!: number; // px

  private SUMMARY_BAR_WIDTH!: number; // px

  private DETAIL_BAR_WIDTH!: number; // px

  private SUMMARY_HIGHLIGHT_LINE_WIDTH!: number; // px

  private DETAIL_RADIUS!: number; // px

  private SUMMARY_RADIUS!: number; // The radius of points in the summary area. (px)

  private MARGIN!: number; // The margin around the details and summary areas. (px)

  private LEFT_MARGIN!: number; // px

  private Y_AXIS_TICK_LENGTH!: number; // px

  private LABEL_FONT_SIZE!: number; // px

  private LABEL_MARGIN!: number; // px

  private LABEL_FONT!: string; // CSS font string.

  private ZOOM_BAR_LINE_WIDTH!: number; // px

  private HOVER_LINE_WIDTH!: number; // px

  private LABEL_COLOR!: string; // CSS color.

  private LABEL_BACKGROUND!: string; // CSS color.

  private CROSSHAIR_COLOR!: string; // CSS color.

  private BAND_COLOR!: string; // CSS color.

  constructor() {
    super(PlotSimpleSk.template);

    this._upgradeProperty('width');
    this._upgradeProperty('height');
    this._upgradeProperty('bands');
    this._upgradeProperty('xbar');
    this._upgradeProperty('hightlight');
    this._upgradeProperty('zoom');

    this.updateScaledMeasurements();
  }

  // Note that in both of the canvas elements we are setting a CSS transform that
  // takes into account window.devicePixelRatio, that is, we are drawing to a
  // scale that matches the displays native resolution and then scaling that back
  // to fit on the page. Also see updateScaledMeasurements for how the device
  // pixel ratio affects all of our pixel calculations.
  private static template = (ele: PlotSimpleSk) => html`
    <canvas
      class="traces"
      width=${ele.width * window.devicePixelRatio}
      height=${ele.height * window.devicePixelRatio}
      style="transform-origin: 0 0; transform: scale(${1
      / window.devicePixelRatio});"
    ></canvas>
    <canvas
      class="overlay"
      width=${ele.width * window.devicePixelRatio}
      height=${ele.height * window.devicePixelRatio}
      style="transform-origin: 0 0; transform: scale(${1
      / window.devicePixelRatio});"
    ></canvas>
  `;

  connectedCallback(): void {
    super.connectedCallback();

    this.render();

    // We need to dynamically resize the canvas elements since they don't do
    // that themselves.
    const resizeObserver = new ResizeObserver((entries: ResizeObserverEntry[]) => {
      entries.forEach((entry) => {
        this.width = entry.contentRect.width;
        this.height = entry.contentRect.height;
      });
    });
    resizeObserver.observe(this);

    this.addEventListener('mousemove', (e) => {
      // Do as little as possible here. The raf() function will periodically
      // check if the mouse has moved and trigger the appropriate redraws.
      this.mouseMoveRaw = {
        clientX: e.clientX,
        clientY: e.clientY,
        shiftKey: e.shiftKey,
      };
    });

    this.addEventListener('mousedown', (e) => {
      const pt = this.eventToCanvasPt(e);
      // If you click in the summary area then begin zooming via drag.
      if (inRect(pt, this.summaryArea.rect!)) {
        const zx = this.summaryArea.range.x.invert(pt.x);
        this.inZoomDrag = 'summary';
        this.zoomBegin = zx;
        this.zoom = [zx, zx + 0.01]; // Add a smidge to the second zx to avoid a degenerate detail plot.

        // Zooming via the summary area clears all details area zooms.
        this.detailsZoomRangesStack = [];
        this.inactiveDetailsZoomRangesStack = [];
      }
      if (inRect(pt, this.detailArea.rect!)) {
        this.inZoomDrag = 'details';
        this.zoomRect = {
          x: pt.x,
          y: pt.y,
          width: 0,
          height: 0,
        };
      }
    });

    this.addEventListener('mouseup', () => {
      if (this.inZoomDrag !== 'no-zoom') {
        this.dispatchZoomEvent();
      }
      if (this.inZoomDrag === 'details') {
        this.doDetailsZoom();
      }
      this.inZoomDrag = 'no-zoom';
    });

    this.addEventListener('mouseleave', () => {
      if (this.inZoomDrag !== 'no-zoom') {
        this.dispatchZoomEvent();
      }
      if (this.inZoomDrag === 'details') {
        this.doDetailsZoom();
      }
      this.inZoomDrag = 'no-zoom';
    });

    this.addEventListener('wheel', (e: WheelEvent) => {
      e.stopPropagation();
      e.preventDefault();
      // If the wheel is spun while we are zoomed then move through the stack of
      // zoom ranges.
      if (this.detailsZoomRangesStack) {
        // Scrolling up on the scroll wheel gives e.deltaY a negative value. Up
        // means to scroll in, which means we want to take a rect from
        // inactiveDetailsZoomRangesStack and make it active by pushing it on
        // detailsZoomRangesStack. Down reverses the push/pop direction.
        if (e.deltaY < 0) {
          if (this.inactiveDetailsZoomRangesStack.length === 0) {
            return;
          }
          this.detailsZoomRangesStack.push(this.inactiveDetailsZoomRangesStack.pop()!);
        } else {
          if (this.detailsZoomRangesStack.length === 0) {
            return;
          }
          this.inactiveDetailsZoomRangesStack.push(this.detailsZoomRangesStack.pop()!);
        }
        this._zoomImpl();
      }
    });

    this.addEventListener('click', (e) => {
      const pt = this.eventToCanvasPt(e);
      if (!inRect(pt, this.detailArea.rect)) {
        return;
      }
      if (!this.pointSearch) {
        return;
      }
      const closest = this.pointSearch.nearest(pt);
      const detail = {
        x: closest.sx,
        y: closest.sy,
        name: closest.name,
      };
      this.dispatchEvent(
        new CustomEvent<PlotSimpleSkTraceEventDetails>('trace_selected', {
          detail,
          bubbles: true,
        }),
      );
    });

    // If the user toggles the theme to/from darkmode then redraw.
    document.addEventListener('theme-chooser-toggle', () => {
      this.render();
    });

    window.requestAnimationFrame(this.raf.bind(this));
  }

  attributeChangedCallback(_: string, oldValue: string, newValue: string): void {
    if (oldValue !== newValue) {
      this.render();
    }
  }

  // Call this when the width or height attrs have changed.
  render(): void {
    this._render();
    const canvas = this.querySelector<HTMLCanvasElement>('canvas.traces')!;
    const overlayCanvas = this.querySelector<HTMLCanvasElement>(
      'canvas.overlay',
    )!;
    if (canvas) {
      this.ctx = canvas.getContext('2d');
      this.overlayCtx = overlayCanvas.getContext('2d');
      this.scale = window.devicePixelRatio;
      this.updateScaledMeasurements();
      this.updateScaleRanges();
      this.recalcDetailPaths();
      this.recalcSummaryPaths();
      this.drawTracesCanvas();
    }
  }

  /**
   * Adds lines to be displayed.
   *
   * Any line id that begins with 'special' will be treated specially,
   * i.e. it will be presented as a dashed black line that doesn't
   * generate events. This may be useful for adding a line at y=0,
   * or a reference trace.
   *
   * @param {Object} lines - A map from trace id to arrays of y values.
   * @param {Array} labels - An array of Date objects the same length as the values.
   *
   */
  addLines(lines: { [key: string]: number[] | null }, labels: Date[]): void {
    const keys = Object.keys(lines);
    if (keys.length === 0) {
      return;
    }
    const startedEmpty = this._zoom === null && this.lineData.length === 0;
    if (labels) {
      this.labels = labels;
    }

    // Convert into the format we will eventually expect.
    keys.forEach((key) => {
      // You can't encode NaN in JSON, so convert sentinel values to NaN here so
      // that dsArray functions will operate correctly.
      lines[key]!.forEach((x, i) => {
        if (x === MISSING_DATA_SENTINEL) {
          lines[key]![i] = NaN;
        }
      });
      const values = lines[key]!;
      this.lineData.push({
        name: key,
        values,
        color: 'black',
        detail: {
          linePath: null,
          dotsPath: null,
        },
        summary: {
          linePath: null,
          dotsPath: null,
        },
      });
    });

    // Set the zoom if we just added data for the first time.
    if (startedEmpty && this.lineData.length > 0) {
      this._zoom = [0, this.lineData[0].values.length - 1];
    }

    this.updateScaleDomains();
    this.recalcSummaryPaths();
    this.recalcDetailPaths();
    this.drawTracesCanvas();
  }

  /**
   * Delete all the lines whose ids are in 'ids' from being plotted.
   *
   * @param {Array<string>} ids - The trace ids to remove.
   */
  deleteLines(ids: string[]): void {
    this.lineData = this.lineData.filter(
      (line) => ids.indexOf(line.name) === -1,
    );

    const onlySpecialLinesRemaining = this.lineData.every((line) => line.name.startsWith(SPECIAL));
    if (onlySpecialLinesRemaining) {
      this.removeAll();
    } else {
      this.updateScaleDomains();
      this.recalcSummaryPaths();
      this.recalcDetailPaths();
      this.drawTracesCanvas();
    }
  }

  /**
   * Remove all lines from plot.
   */
  removeAll(): void {
    this.lineData = [];
    this.labels = [];
    this.hoverPt = {
      x: -1,
      y: -1,
      name: '',
    };
    this.pointSearch = null;
    this.crosshair = {
      x: -1,
      y: -1,
      shift: false,
    };
    this.mouseMoveRaw = null;
    this.highlighted = {};
    this._xbar = -1;
    this._zoom = null;
    this.inZoomDrag = 'no-zoom';
    this.detailsZoomRangesStack = [];
    this.inactiveDetailsZoomRangesStack = [];
    this.drawTracesCanvas();
  }

  /**
   * Return the names of all the lines being plotted, not including SPECIAL
   * names.
   * */
  getLineNames(): string[] {
    const ret: string[] = [];
    this.lineData.forEach((line) => {
      if (line.name.startsWith(SPECIAL)) {
        return;
      }
      ret.push(line.name);
    });
    return ret;
  }

  /**
   * Update all the things that look like constants, but are really dependent on
   * window.devicePixelRatio or the current CSS styling.
   */
  private updateScaledMeasurements() {
    // The height of the summary area.
    if (this.summary) {
      this.SUMMARY_HEIGHT = 50 * this.scale; // px
    } else {
      this.SUMMARY_HEIGHT = 0;
    }

    this.SUMMARY_BAR_WIDTH = 2 * this.scale; // px

    this.DETAIL_BAR_WIDTH = 3 * this.scale; // px

    this.SUMMARY_HIGHLIGHT_LINE_WIDTH = 3 * this.scale;

    // The radius of points in the details area.
    this.DETAIL_RADIUS = 3 * this.scale; // px

    // The radius of points in the summary area.
    this.SUMMARY_RADIUS = 2 * this.scale; // px

    // The margin around the details and summary areas.
    this.MARGIN = 32 * this.scale; // px

    this.LEFT_MARGIN = 2 * this.MARGIN; // px

    this.Y_AXIS_TICK_LENGTH = this.MARGIN / 4; // px

    this.LABEL_FONT_SIZE = 14 * this.scale; // px

    this.LABEL_MARGIN = 6 * this.scale; // px

    this.LABEL_FONT = `${this.LABEL_FONT_SIZE}px Roboto,Helvetica,Arial,Bitstream Vera Sans,sans-serif`;

    this.ZOOM_BAR_LINE_WIDTH = 3 * this.scale; // px

    this.HOVER_LINE_WIDTH = 1 * this.scale; // px

    this.CROSSHAIR_COLOR = '#f00';

    this.BAND_COLOR = '#888';

    // Pull out the computed colors.
    const style = getComputedStyle(this);

    // Start by using the computed colors.
    this.LABEL_COLOR = style.color;
    this.LABEL_BACKGROUND = style.backgroundColor;

    // Now override with CSS variables if they are present.
    const onBackground = style.getPropertyValue('--on-backgroud');
    if (onBackground !== '') {
      this.LABEL_COLOR = onBackground;
    }

    const background = style.getPropertyValue('--backgroud');
    if (background !== '') {
      this.LABEL_BACKGROUND = background;
    }

    const errorColor = style.getPropertyValue('--error');
    if (errorColor !== '') {
      this.CROSSHAIR_COLOR = errorColor;
    }

    const secondaryColor = style.getPropertyValue('--secondary');
    if (secondaryColor !== '') {
      this.BAND_COLOR = secondaryColor;
    }
  }

  private dispatchZoomEvent() {
    if (!this._zoom) {
      return;
    }
    let beginIndex = Math.floor(this._zoom[0] - 0.1);
    if (beginIndex < 0) {
      beginIndex = 0;
    }
    let endIndex = Math.ceil(this._zoom[1] + 0.1);
    if (endIndex > this.labels.length - 1) {
      endIndex = this.labels.length - 1;
    }
    const detail = {
      xBegin: this.labels[beginIndex],
      xEnd: this.labels[endIndex],
    };
    this.dispatchEvent(
      new CustomEvent<PlotSimpleSkZoomEventDetails>('zoom', {
        detail,
        bubbles: true,
      }),
    );
  }

  /**
   * Convert mouse event coordinates to a canvas point.
   *
   * @param {Object} e - A mouse event or an object that has the coords stored
   * in clientX and clientY.
   */
  private eventToCanvasPt(e: MouseMoveRaw) {
    const clientRect = this.ctx!.canvas.getBoundingClientRect();
    return {
      x: (e.clientX - clientRect.left) * this.scale,
      y: (e.clientY - clientRect.top) * this.scale,
    };
  }

  // Handles requestAnimationFrame callbacks.
  private raf() {
    // Always queue up our next raf first.
    window.requestAnimationFrame(() => this.raf());

    // Bail out early if the mouse hasn't moved.
    if (this.mouseMoveRaw === null) {
      return;
    }
    if (this.inZoomDrag === 'no-zoom') {
      const pt = this.eventToCanvasPt(this.mouseMoveRaw);

      // Update _hoverPt if needed.
      if (this.pointSearch) {
        const closest = this.pointSearch.nearest(pt);
        const detail = {
          x: closest.sx,
          y: closest.sy,
          name: closest.name,
        };
        if (detail.x !== this.hoverPt.x || detail.y !== this.hoverPt.y) {
          this.hoverPt = detail;
          this.dispatchEvent(
            new CustomEvent<PlotSimpleSkTraceEventDetails>('trace_focused', {
              detail,
              bubbles: true,
            }),
          );
        }
      }

      // Update crosshair.
      if (this.mouseMoveRaw.shiftKey && this.pointSearch) {
        this.crosshair = {
          x: pt.x,
          y: pt.y,
          shift: false,
        };
        clampToRect(this.crosshair, this.detailArea.rect);
      } else {
        this.crosshair = {
          x: this.detailArea.range.x(this.hoverPt.x),
          y: this.detailArea.range.y(this.hoverPt.y),
          shift: true,
        };
      }
      this.drawOverlayCanvas();
    } else {
      // We are zooming.
      const pt = this.eventToCanvasPt(this.mouseMoveRaw);

      if (this.inZoomDrag === 'summary') {
        clampToRect(pt, this.summaryArea.rect);

        // x in source coordinates.
        const sx = this.summaryArea.range.x.invert(pt.x);

        // Set zoom, always making sure we go from lowest to highest.
        let zoom: ZoomRange = [this.zoomBegin, sx];
        if (this.zoomBegin > sx) {
          zoom = [sx, this.zoomBegin];
        }
        this.zoom = zoom;
      } else if (this.inZoomDrag === 'details') {
        clampToRect(pt, this.detailArea.rect);
        this.zoomRect.width = pt.x - this.zoomRect.x;
        this.zoomRect.height = pt.y - this.zoomRect.y;
        this.drawOverlayCanvas();
      }
    }
    this.mouseMoveRaw = null;
  }

  /**
   * This is a super simple hash (h = h * 31 + x_i) currently used
   * for things like assigning colors to graphs based on trace ids. It
   * shouldn't be used for anything more serious than that.
   *
   * @param {String} s - A string to hash.
   * @return {Number} A 32 bit hash for the given string.
   */
  private hashString(s: string) {
    let hash = 0;
    for (let i = s.length - 1; i >= 0; i--) {
      // eslint-disable-next-line no-bitwise
      hash = (hash << 5) - hash + s.charCodeAt(i);
      // eslint-disable-next-line no-bitwise
      hash |= 0;
    }
    return Math.abs(hash);
  }

  // Rebuilds our cache of Path2D objects we use for quick rendering.
  private recalcSummaryPaths() {
    this.lineData.forEach((line) => {
      // Need to pass in the x and y ranges, and the dot radius.
      if (line.name.startsWith(SPECIAL)) {
        line.color = this.LABEL_COLOR;
      } else {
        line.color = COLORS[(this.hashString(line.name) % 8) + 1];
      }

      const summaryBuilder = new PathBuilder(
        this.summaryArea.range.x,
        this.summaryArea.range.y,
        this.SUMMARY_RADIUS,
      );

      line.values.forEach((y, x) => {
        if (Number.isNaN(y)) {
          return;
        }
        summaryBuilder.add(x, y);
      });
      line.summary = summaryBuilder.paths();
    });

    // Build summary x-axis.
    this.recalcXAxis(this.summaryArea, this.labels, 0);
  }

  // Rebuilds our cache of Path2D objects we use for quick rendering.
  private recalcDetailPaths() {
    const domain = this.detailArea.range.x.domain();
    domain[0] = Math.floor(domain[0] - 0.1);
    domain[1] = Math.ceil(domain[1] + 0.1);
    this.lineData.forEach((line) => {
      // Need to pass in the x and y ranges, and the dot radius.
      if (line.name.startsWith(SPECIAL)) {
        line.color = this.LABEL_COLOR;
      } else {
        line.color = COLORS[(this.hashString(line.name) % 8) + 1];
      }

      const detailBuilder = new PathBuilder(
        this.detailArea.range.x,
        this.detailArea.range.y,
        this.DETAIL_RADIUS,
      );

      let previousPoint: Point = invalidPoint;
      let addedPointFromBeforeTheDomain = false;
      let addedPointFromAfterTheDomain = false;
      line.values.forEach((y, x) => {
        if (Number.isNaN(y)) {
          return;
        }
        // Always add in one point after the domain so we draw all visible line
        // segments.
        if (x > domain[1] && !addedPointFromAfterTheDomain) {
          detailBuilder.add(x, y);
          addedPointFromAfterTheDomain = true;
          return;
        }
        if (x < domain[0] || x > domain[1]) {
          previousPoint = { x: x, y: y };
          return;
        }
        // Always add in one point before the domain so we draw all visible line
        // segments.
        if (!addedPointFromBeforeTheDomain) {
          if (pointIsValid(previousPoint)) {
            detailBuilder.add(previousPoint.x, previousPoint.y);
          }
          addedPointFromBeforeTheDomain = true;
        }
        detailBuilder.add(x, y);
      });
      line.detail = detailBuilder.paths();
    });

    // Build detail x-axis.
    const detailDomain = this.detailArea.range.x.domain();
    const labelOffset = Math.ceil(detailDomain[0]);
    const detailLabels = this.labels.slice(
      Math.ceil(detailDomain[0]),
      Math.floor(detailDomain[1] + 1),
    );
    this.recalcXAxis(this.detailArea, detailLabels, labelOffset);

    // Build detail y-axis.
    this.recalcYAxis(this.detailArea);
    this.recalcSearch();
  }

  // Recalculates the y-axis info.
  private recalcYAxis(area: DetailArea) {
    const yAxisPath = new Path2D();
    const thinX = Math.floor(this.detailArea.rect.x) + 0.5; // Make sure we get a thin line. https://developer.mozilla.org/en-US/docs/Web/API/Canvas_API/Tutorial/Applying_styles_and_colors#A_lineWidth_example
    yAxisPath.moveTo(thinX, this.detailArea.rect.y);
    yAxisPath.lineTo(thinX, this.detailArea.rect.y + this.detailArea.rect.height);
    area.yaxis.labels = [];
    area.range.y.ticks(NUM_Y_TICKS).forEach((t) => {
      const label = {
        x: 0,
        y: Math.floor(area.range.y(t)) + 0.5,
        text: `${this.numberFormatter.format(t)}`,
      };
      area.yaxis.labels.push(label);
      yAxisPath.moveTo(thinX, label.y);
      yAxisPath.lineTo(thinX - this.Y_AXIS_TICK_LENGTH, label.y);
    });
    area.yaxis.path = yAxisPath;
  }

  // Recalculates the x-axis info.
  private recalcXAxis(area: Area, labels: Date[], labelOffset: number) {
    const xAxisPath = new Path2D();
    const thinY = Math.floor(area.rect.y) + 0.5; // Make sure we get a thin line.
    xAxisPath.moveTo(area.rect.x + 0.5, thinY);
    xAxisPath.lineTo(area.rect.x + 0.5 + area.rect.width, thinY);
    area.axis.labels = [];
    ticks(labels).forEach((tick) => {
      const label = {
        x: Math.floor(area.range.x(tick.x + labelOffset)) + 0.5,
        y: area.rect.y - this.MARGIN / 2,
        text: tick.text,
      };
      area.axis.labels.push(label);
      xAxisPath.moveTo(label.x, area.rect.y);
      xAxisPath.lineTo(label.x, area.rect.y - this.MARGIN / 2);
    });
    area.axis.path = xAxisPath;
  }

  // Rebuilds the kdTree we use to look up closest points.
  private recalcSearch() {
    if (this.recalcSearchTask) {
      return;
    }
    this.recalcSearchTask = window.setTimeout(() => this.recalcSearchImpl());
  }

  private recalcSearchImpl() {
    if (this.zoomTask) {
      // If there is a pending zoom task then let that complete first since zooming
      // invalidates the search tree and it needs to be built again.
      this.recalcSearchTask = window.setTimeout(() => this.recalcSearchImpl());
      return;
    }
    const domain = this.detailArea.range.x.domain();
    domain[0] = Math.floor(domain[0] - 0.1);
    domain[1] = Math.ceil(domain[1] + 0.1);
    const searchBuilder = new SearchBuilder(
      this.detailArea.range.x,
      this.detailArea.range.y,
    );
    this.lineData.forEach((line) => {
      line.values.forEach((y, x) => {
        if (Number.isNaN(y)) {
          return;
        }
        if (x < domain[0] || x > domain[1]) {
          return;
        }
        searchBuilder.add(x, y, line.name);
      });
    });
    this.pointSearch = searchBuilder.kdTree();
    this.recalcSearchTask = 0;
  }

  // Updates all of our d3Scale domains.
  private updateScaleDomains() {
    let domainEnd = 1;
    if (this.lineData && this.lineData.length) {
      domainEnd = this.lineData[0].values.length - 1;
    }
    if (this._zoom) {
      this.detailArea.range.x = this.detailArea.range.x.domain(this._zoom);
    } else {
      this.detailArea.range.x = this.detailArea.range.x.domain([0, domainEnd]);
    }

    this.summaryArea.range.x = this.summaryArea.range.x.domain([0, domainEnd]);

    const domain = [
      d3Array.min(this.lineData, (line) => d3Array.min(line.values))!,
      d3Array.max(this.lineData, (line) => d3Array.max(line.values))!,
    ];

    this.detailArea.range.y = this.detailArea.range.y.domain(domain).nice();

    this.summaryArea.range.y = this.summaryArea.range.y.domain(domain);

    // If detailsZoomRangeStacks is not empty then it overrides the detail
    // range.
    if (this.detailsZoomRangesStack.length > 0) {
      const zoom = this.detailsZoomRangesStack[this.detailsZoomRangesStack.length - 1];
      this.detailArea.range.x = this.detailArea.range.x.domain([zoom.x, zoom.x + zoom.width]);
      this.detailArea.range.y = this.detailArea.range.y.domain([zoom.y, zoom.y + zoom.height]);
    }
  }

  // Updates all of our d3Scale ranges. Also updates detail and summary rects.
  private updateScaleRanges() {
    const width = this.ctx!.canvas.width;
    const height = this.ctx!.canvas.height;

    this.summaryArea.range.x = this.summaryArea.range.x.range([
      this.LEFT_MARGIN,
      width - this.MARGIN,
    ]);

    this.summaryArea.range.y = this.summaryArea.range.y.range([
      this.SUMMARY_HEIGHT + this.MARGIN,
      this.MARGIN,
    ]);

    this.detailArea.range.x = this.detailArea.range.x.range([
      this.LEFT_MARGIN,
      width - this.MARGIN,
    ]);

    this.detailArea.range.y = this.detailArea.range.y.range([
      height - this.MARGIN,
      this.SUMMARY_HEIGHT + 2 * this.MARGIN,
    ]);

    this.summaryArea.rect = {
      x: this.LEFT_MARGIN,
      y: this.MARGIN,
      width: width - this.MARGIN - this.LEFT_MARGIN,
      height: this.SUMMARY_HEIGHT,
    };

    this.detailArea.rect = {
      x: this.LEFT_MARGIN,
      y: this.SUMMARY_HEIGHT + 2 * this.MARGIN,
      width: width - this.MARGIN - this.LEFT_MARGIN,
      height: height - this.SUMMARY_HEIGHT - 3 * this.MARGIN,
    };
  }

  private doDetailsZoom() {
    // Don't actually do the zoom if the box isn't big enough.
    if (
      Math.abs(this.zoomRect.width) < MIN_MOUSE_MOVE_FOR_ZOOM
      || Math.abs(this.zoomRect.height) < MIN_MOUSE_MOVE_FOR_ZOOM
    ) {
      return;
    }
    this.detailsZoomRangesStack.push(
      rectFromRangeInvert(this.detailArea.range, this.zoomRect),
    );

    // We added a new zoom range, which means all the inactive zoom ranges are
    // no longer valid.
    this.inactiveDetailsZoomRangesStack = [];
    this.inZoomDrag = 'no-zoom';
    this._zoomImpl();
  }

  // Draw the contents of the overlay canvas.
  private drawOverlayCanvas() {
    // Always start by clearing the overlay.
    const width = this.overlayCtx!.canvas.width;
    const height = this.overlayCtx!.canvas.height;
    const ctx = this.overlayCtx!;

    ctx.clearRect(0, 0, width, height);

    if (this.summary) {
      // First clip to the summary region.
      ctx.save();
      {
        // Block to scope save/restore.
        clipToRect(ctx, this.summaryArea.rect);

        // Draw the xbar.
        this.drawXBar(ctx, this.summaryArea, this.SUMMARY_BAR_WIDTH);

        // Draw the bands.
        this.drawBands(ctx, this.summaryArea, this.SUMMARY_BAR_WIDTH);

        // If detailsZoomRangeStacks is not empty then draw a box to indicate
        // the zoomed region.
        if (this.detailsZoomRangesStack.length > 0) {
          const zoom = this.detailsZoomRangesStack[this.detailsZoomRangesStack.length - 1];
          this.drawZoomRect(ctx, rectFromRange(this.summaryArea.range, zoom));
        }

        // Draw the zoom on the summary.
        if (this._zoom !== null) {
          ctx.lineWidth = this.ZOOM_BAR_LINE_WIDTH;
          ctx.strokeStyle = this.LABEL_COLOR;

          // Draw left bar.
          const leftx = this.summaryArea.range.x(this._zoom[0]);
          ctx.beginPath();
          ctx.moveTo(leftx, this.summaryArea.rect.y);
          ctx.lineTo(leftx, this.summaryArea.rect.y + this.summaryArea.rect.height);

          // Draw right bar.
          const rightx = this.summaryArea.range.x(this._zoom[1]);
          ctx.moveTo(rightx, this.summaryArea.rect.y);
          ctx.lineTo(rightx, this.summaryArea.rect.y + this.summaryArea.rect.height);
          ctx.stroke();

          // Draw gray boxes.
          ctx.fillStyle = ZOOM_RECT_COLOR;
          ctx.rect(
            this.summaryArea.rect.x,
            this.summaryArea.rect.y,
            leftx - this.summaryArea.rect.x,
            this.summaryArea.rect.height,
          );
          ctx.rect(
            rightx,
            this.summaryArea.rect.y,
            this.summaryArea.rect.x + this.summaryArea.rect.width - rightx,
            this.summaryArea.rect.height,
          );

          ctx.fill();
        }
      }
      ctx.restore();
    }

    // Now clip to the detail region.
    ctx.save();
    {
      // Block to scope save/restore.
      clipToRect(ctx, this.detailArea.rect);

      // Draw the xbar.
      this.drawXBar(ctx, this.detailArea, this.DETAIL_BAR_WIDTH);

      // Draw the bands.
      this.drawBands(ctx, this.detailArea, this.DETAIL_BAR_WIDTH);

      // Draw highlighted lines.
      this.lineData.forEach((highlightedLine) => {
        if (!(highlightedLine.name in this.highlighted)) {
          return;
        }
        ctx.strokeStyle = highlightedLine.color;
        ctx.fillStyle = this.LABEL_BACKGROUND;
        ctx.lineWidth = this.SUMMARY_HIGHLIGHT_LINE_WIDTH;

        ctx.stroke(highlightedLine.detail.linePath!);
        if (this.dots) {
          ctx.fill(highlightedLine.detail.dotsPath!);
          ctx.stroke(highlightedLine.detail.dotsPath!);
        }
      });

      // Find the line currently hovered over.
      let line = null;
      for (let i = 0; i < this.lineData.length; i++) {
        if (this.lineData[i].name === this.hoverPt.name) {
          line = this.lineData[i];
          break;
        }
      }
      if (line !== null) {
        // Draw the hovered line and dots in a different color.
        ctx.strokeStyle = HOVER_COLOR;
        ctx.fillStyle = HOVER_COLOR;
        ctx.lineWidth = this.HOVER_LINE_WIDTH;

        // Just draw the dots, not the line.
        if (this.dots) {
          ctx.fill(line.detail.dotsPath!);
          ctx.stroke(line.detail.dotsPath!);
        }
      }

      if (this.inZoomDrag === 'details') {
        this.drawZoomRect(ctx, this.zoomRect);
      } else if (this.inZoomDrag === 'no-zoom') {
        // Draw the crosshairs.
        ctx.strokeStyle = this.CROSSHAIR_COLOR;
        ctx.lineWidth = AXIS_LINE_WIDTH;
        ctx.beginPath();
        const thinX = Math.floor(this.crosshair.x) + 0.5; // Make sure we get a thin line.
        const thinY = Math.floor(this.crosshair.y) + 0.5; // Make sure we get a thin line.
        ctx.moveTo(this.detailArea.rect.x, thinY);
        ctx.lineTo(this.detailArea.rect.x + this.detailArea.rect.width, thinY);
        ctx.moveTo(thinX, this.detailArea.rect.y);
        ctx.lineTo(thinX, this.detailArea.rect.y + this.detailArea.rect.height);
        ctx.stroke();

        // Y label at crosshair if shift is pressed.
        if (this.crosshair.shift) {
          // Draw the label offset from the crosshair.
          ctx.font = this.LABEL_FONT;
          ctx.textBaseline = 'bottom';
          const label = this.numberFormatter.format(this.hoverPt.y);
          let x = this.crosshair.x + this.MARGIN;
          let y = this.crosshair.y - this.MARGIN;

          // First draw a white backdrop.
          ctx.fillStyle = this.LABEL_BACKGROUND;
          const meas = ctx.measureText(label);
          const labelHeight = this.LABEL_FONT_SIZE + 2 * this.LABEL_MARGIN;
          const labelWidth = meas.width + this.LABEL_MARGIN * 2;

          // Bump the text to different quadrants so it is always visible.
          if (y < this.detailArea.rect.y + this.detailArea.rect.height / 2) {
            y = this.crosshair.y + this.MARGIN;
          }
          if (x > this.detailArea.rect.x + this.detailArea.rect.width / 2) {
            x = x - labelWidth - 2 * this.MARGIN;
          }

          ctx.beginPath();
          ctx.rect(
            x - this.LABEL_MARGIN,
            y + this.LABEL_MARGIN,
            labelWidth,
            -labelHeight,
          );
          ctx.fill();
          ctx.strokeStyle = this.LABEL_COLOR;
          ctx.beginPath();
          ctx.rect(
            x - this.LABEL_MARGIN,
            y + this.LABEL_MARGIN,
            labelWidth,
            -labelHeight,
          );
          ctx.stroke();

          // Now draw text on top.
          ctx.fillStyle = this.LABEL_COLOR;
          ctx.fillText(label, x, y);
        }
      }
    }
    ctx.restore();
  }

  // Draw a dashed rectangle for the details zoom.
  private drawZoomRect(ctx: CanvasRenderingContext2D, rect: Rect) {
    ctx.strokeStyle = this.LABEL_COLOR;
    ctx.lineWidth = 1;
    ctx.setLineDash([2, 2]);
    ctx.strokeRect(rect.x, rect.y, rect.width, rect.height);
    ctx.setLineDash([]);
  }

  // Draw the xbar in the given area with the given width.
  private drawXBar(ctx: CanvasRenderingContext2D, area: Area, width: number) {
    if (this.xbar === -1) {
      return;
    }
    ctx.lineWidth = width;
    ctx.strokeStyle = this.CROSSHAIR_COLOR;
    const bx = area.range.x(this._xbar);
    ctx.beginPath();
    ctx.moveTo(bx, area.rect.y);
    ctx.lineTo(bx, area.rect.y + area.rect.height);
    ctx.stroke();
  }

  // Draw the bands in the given area with the given width.
  private drawBands(ctx: CanvasRenderingContext2D, area: Area, width: number) {
    ctx.lineWidth = width;
    ctx.strokeStyle = this.BAND_COLOR;
    ctx.setLineDash([width, width]);
    ctx.beginPath();
    this._bands.forEach((band) => {
      const bx = area.range.x(band);
      ctx.moveTo(bx, area.rect.y);
      ctx.lineTo(bx, area.rect.y + area.rect.height);
    });
    ctx.stroke();
    ctx.setLineDash([]);
  }

  // Draw everything on the trace canvas.
  //
  // Well, not quite everything, if we are drag zooming then we only redraw the
  // details and not the summary.
  private drawTracesCanvas() {
    const width = this.ctx!.canvas.width;
    const height = this.ctx!.canvas.height;
    const ctx = this.ctx!;

    if (this.inZoomDrag !== 'no-zoom') {
      ctx.clearRect(
        this.detailArea.rect.x - this.MARGIN,
        this.detailArea.rect.y - this.MARGIN,
        this.detailArea.rect.width + 2 * this.MARGIN,
        this.detailArea.rect.height + 2 * this.MARGIN,
      );
    } else {
      ctx.clearRect(0, 0, width, height);
    }
    ctx.fillStyle = this.LABEL_BACKGROUND;

    // Draw the detail.
    ctx.save();
    {
      // Block to scope save/restore.
      clipToRect(ctx, this.detailArea.rect);
      this.drawXAxis(ctx, this.detailArea);
      ctx.fillStyle = this.LABEL_BACKGROUND;

      this.lineData.forEach((line) => {
        ctx.strokeStyle = line.color;
        ctx.lineWidth = DETAIL_LINE_WIDTH;
        ctx.stroke(line.detail.linePath!);
        if (this.dots) {
          ctx.fill(line.detail.dotsPath!);
          ctx.stroke(line.detail.dotsPath!);
        }
      });
    }
    ctx.restore();
    this.drawXAxis(ctx, this.detailArea);

    if (this.inZoomDrag === 'no-zoom' && this.summary) {
      // Draw the summary.
      ctx.save();
      {
        // Block to scope save/restore.
        clipToRect(ctx, this.summaryArea.rect);
        this.lineData.forEach((line) => {
          ctx.fillStyle = this.LABEL_BACKGROUND;
          ctx.strokeStyle = line.color;
          ctx.lineWidth = SUMMARY_LINE_WIDTH;
          ctx.stroke(line.summary.linePath!);
          if (this.dots) {
            ctx.fill(line.summary.dotsPath!);
            ctx.stroke(line.summary.dotsPath!);
          }
        });
      }
      ctx.restore();
      this.drawXAxis(ctx, this.summaryArea);
    }
    // Draw y-Axes.
    this.drawYAxis(ctx, this.detailArea);

    this.drawOverlayCanvas();
  }

  // Draw a y-axis using the given context in the given area.
  private drawYAxis(ctx: CanvasRenderingContext2D, area: DetailArea) {
    ctx.strokeStyle = this.LABEL_COLOR;
    ctx.fillStyle = this.LABEL_COLOR;
    ctx.font = this.LABEL_FONT;
    ctx.textBaseline = 'middle';
    ctx.lineWidth = AXIS_LINE_WIDTH;
    ctx.textAlign = 'right';
    ctx.stroke(area.yaxis.path);
    const labelWidth = (3 * this.LEFT_MARGIN) / 4;
    area.yaxis.labels.forEach((label) => {
      ctx.fillText(label.text, label.x + labelWidth, label.y, labelWidth);
    });
  }

  // Draw a x-axis using the given context in the given area.
  private drawXAxis(ctx: CanvasRenderingContext2D, area: Area) {
    ctx.strokeStyle = this.LABEL_COLOR;
    ctx.fillStyle = this.LABEL_COLOR;
    ctx.font = this.LABEL_FONT;
    ctx.textBaseline = 'middle';
    ctx.lineWidth = AXIS_LINE_WIDTH;
    ctx.stroke(area.axis.path);
    area.axis.labels.forEach((label) => {
      ctx.fillText(label.text, label.x - 2, label.y);
    });
  }

  /**
   *  An array of trace ids to highlight. Set to [] to remove all highlighting.
   */
  get highlight(): string[] {
    return Object.keys(this.highlighted);
  }

  set highlight(ids: string[]) {
    this.highlighted = {};
    ids.forEach((name) => {
      this.highlighted[name] = true;
    });
    this.drawOverlayCanvas();
  }

  /**
   * Location to put a vertical marking bar on the graph. Can be set to -1 to
   * not display any bar.
   */
  get xbar(): number {
    return this._xbar;
  }

  set xbar(value: number) {
    this._xbar = value;
    this.drawOverlayCanvas();
  }

  /**
   * A list of x source offsets to place vertical markers. into labels. Can be
   *   set to [] to remove all bands.
   */
  get bands(): number[] {
    return this._bands;
  }

  set bands(bands: number[]) {
    if (!bands) {
      this._bands = [];
    } else {
      this._bands = bands;
    }
    this.drawOverlayCanvas();
  }

  /** The zoom range, an array of two values in source x units. Can be set to
   * null to have no zoom.
   */
  get zoom(): ZoomRange {
    return this._zoom;
  }

  set zoom(range: ZoomRange) {
    this._zoom = range;
    if (this.zoomTask) {
      return;
    }
    this.zoomTask = window.setTimeout(() => this._zoomImpl());
  }

  private _zoomImpl() {
    this.updateScaleDomains();
    this.recalcDetailPaths();
    this.drawTracesCanvas();
    this.zoomTask = 0;
  }

  static get observedAttributes(): string[] {
    return ['width', 'height', 'summary'];
  }

  /** Mirrors the width attribute. */
  get width(): number {
    return +(this.getAttribute('width') || '0');
  }

  set width(val: number) {
    this.setAttribute('width', val.toString());
  }

  /** Mirrors the height attribute. */
  get height(): number {
    return +(this.getAttribute('height') || '0');
  }

  set height(val: number) {
    this.setAttribute('height', val.toString());
  }

  /** @prop summary {string} Mirrors the summary attribute. */
  get summary(): boolean {
    return this.hasAttribute('summary');
  }

  set summary(val: boolean) {
    if (val) {
      this.setAttribute('summary', val.toString());
    } else {
      this.removeAttribute('summary');
    }
  }

  /** @prop nodots {boolean} Mirrors the nodots attribute.  */
  get dots(): boolean { return this._dots; }

  set dots(val: boolean) {
    this._dots = val;
    this.drawTracesCanvas();
  }
}

define('plot-simple-sk', PlotSimpleSk);
