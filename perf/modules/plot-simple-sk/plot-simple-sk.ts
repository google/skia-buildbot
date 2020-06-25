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
 *    | 2 |          Details                           |   | | x |
 *    |   | | M |                                            | M | | A |
 *    | A | | R |                                            | R | | G |
 *    | G | | I |                                            | I | | N |
 *    | N |
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
import * as ResizeObserverPolyfill from 'resize-observer-polyfill';
// This import is needed because https://github.com/Microsoft/TypeScript/issues/28502
import ResizeObserver from 'resize-observer-polyfill';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { KDTree, KDPoint } from './kd';
import { ticks } from './ticks';

//  Prefix for trace ids that are not real traces, such as special_zero. Special
//  traces never receive focus and can't be clicked on.
const SPECIAL = 'special';

const MISSING_DATA_SENTINEL = 1e32;

const NUM_Y_TICKS = 4;

const HOVER_COLOR = '#8887'; // Note the alpha value.

const ZOOM_RECT_COLOR = '#0007'; // Note the alpha value.

const SUMMARY_LINE_WIDTH = 1; // px

const DETAIL_LINE_WIDTH = 1; // px

const AXIS_LINE_WIDTH = 1; // px

// As backup use a Polyfill for ResizeObserver if it isn't supported. This can
// go away when Safari supports ResizeObserver:
// https://caniuse.com/#feat=resizeobserver
const LocalResizeObserver = ResizeObserver || ResizeObserverPolyfill;

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
interface Range {
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
    radius: number
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

interface Rect extends Point {
  width: number;
  height: number;
}

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
    yRange: d3Scale.ScaleLinear<number, number>
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
      name: name,
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
    pt.x >= rect.x &&
    pt.x < rect.x + rect.width &&
    pt.y >= rect.y &&
    pt.y < rect.y + rect.height
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

interface SummaryArea extends Area { }

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

export class PlotSimpleSk extends ElementSk {
  // Note that in both of the canvas elements we are setting a CSS transform that
  // takes into account window.devicePixelRatio, that is, we are drawing to a
  // scale that matches the displays native resolution and then scaling that back
  // to fit on the page. Also see _updateScaledMeasurements for how the device
  // pixel ratio affects all of our pixel calculations.
  private static template = (ele: PlotSimpleSk) => html` <canvas
      class="traces"
      width=${ele.width * window.devicePixelRatio}
      height=${ele.height * window.devicePixelRatio}
      style="transform-origin: 0 0; transform: scale(${1 /
    window.devicePixelRatio});"
    ></canvas>
    <canvas
      class="overlay"
      width=${ele.width * window.devicePixelRatio}
      height=${ele.height * window.devicePixelRatio}
      style="transform-origin: 0 0; transform: scale(${1 /
    window.devicePixelRatio});"
    ></canvas>`;

  // The location of the XBar. See the xbar property..
  private _xbar: number;

  // The locations of the background bands. See bands property.
  private _bands: number[];

  // A map of trace names to 'true' of traces that are highlighted.
  private _highlighted: { [key: string]: boolean };

  // The data we are plotting.
  // An array of objects of this form:
  //   {
  //     name: key,
  //     values: [1.0, 1.1, 0.9, ...],
  //     detail: {
  //       linePath: Path2D,
  //       dotsPath: Path2D,
  //     },
  //     summary: {
  //       linePath: Path2D,
  //       dotsPath: Path2D,
  //     },
  //   }
  private _lineData: LineData[];

  // An array of Date()'s the same length as the values in _lineData.
  private _labels: Date[];

  // The current zoom, either null or an array of two values in source x
  // coordinates, e.g. [1, 12].
  private _zoom: ZoomRange;

  // The source coordinate where a zoom started.
  private _zoomBegin: number;

  // True if we are currently drag zooming, i.e. the mouse is pressed and
  // moving over the summary.
  private _inZoomDrag: boolean;

  // The Canvas 2D context of the traces canvas.
  private _ctx: CanvasRenderingContext2D | null;

  // The Canvas 2D context of the overlay canvas.
  private _overlayCtx: CanvasRenderingContext2D | null;

  // The window.devicePixelRatio.
  private _scale: number;

  // A copy of the clientX, clientY, and shiftKey values of mousemove events,
  // or null if a mousemove event hasn't occurred since the last time it was
  // processed.
  private _mouseMoveRaw: MouseMoveRaw | null;

  // A kdTree for all the points being displayed, in source coordinates. Is
  // null if no traces are being displayed.
  private _pointSearch: KDTree<SearchPoint> | null;

  // The closest trace point to the mouse. May be {} if no traces are
  // displayed or the mouse hasn't moved over the canvas yet. Has the form:
  //   {
  //     x: x,
  //     y: y,
  //     name: String, // name of trace
  //   }
  private _hoverPt: HoverPoint;

  // The location of the crosshair in canvas coordinates. Of the form:
  //   {
  //     x: x,
  //     y: y,
  //     shift: Boolean,
  //   }
  //
  // The value of shift is true of the shift key is being pressed while the
  // mouse moves.
  // }
  private _crosshair: CrosshairPoint;

  // All the info we need about the summary area.
  private _summary: SummaryArea;

  // All the info we need about the details area.
  private _detail: DetailArea;

  // The total number of points we are displaying. Used to decide whether or
  // not to update the details traces when zooming.
  private _numPoints: number;

  // A task to rebuild the k-d search tree used for finding the closest point
  // to the mouse. The actual value is a window.setTimer timerId or zero if no
  // task is scheduled.
  //
  // See https://jakearchibald.com/2015/tasks-microtasks-queues-and-schedules/
  // for details on tasks vs microtasks.
  private _recalcSearchTask: number;

  // A task to do the actual re-draw work of a zoom. The actual value is a
  // window.setTimer timerId or zero if no task is scheduled.
  //
  // See https://jakearchibald.com/2015/tasks-microtasks-queues-and-schedules/
  // for details on tasks vs microtasks.
  private _zoomTask: number;

  // A formatter that prints numbers nicely, such as adding commas. Used when
  // display the hover text.
  private _numberFormatter: Intl.NumberFormat;

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

    // The location of the XBar. See the xbar property..
    this._xbar = -1;

    // The locations of the background bands. See bands property.
    this._bands = [];

    // A map of trace names to 'true' of traces that are highlighted.
    this._highlighted = {};

    // The data we are plotting.
    // An array of objects of this form:
    //   {
    //     name: key,
    //     values: [1.0, 1.1, 0.9, ...],
    //     detail: {
    //       linePath: Path2D,
    //       dotsPath: Path2D,
    //     },
    //     summary: {
    //       linePath: Path2D,
    //       dotsPath: Path2D,
    //     },
    //   }
    this._lineData = [];

    // An array of Date()'s the same length as the values in _lineData.
    this._labels = [];

    // The current zoom, either null or an array of two values in source x
    // coordinates, e.g. [1, 12].
    this._zoom = null;

    // True if we are currently drag zooming, i.e. the mouse is pressed and
    // moving over the summary.
    this._inZoomDrag = false;

    this._zoomBegin = 0;

    // The Canvas 2D context of the traces canvas.
    this._ctx = null;

    // The Canvas 2D context of the overlay canvas.
    this._overlayCtx = null;

    // The window.devicePixelRatio.
    this._scale = 1.0;

    // A copy of the clientX, clientY, and shiftKey values of mousemove events,
    // or null if a mousemove event hasn't occurred since the last time it was
    // processed.
    this._mouseMoveRaw = null;

    // A kdTree for all the points being displayed, in source coordinates. Is
    // null if no traces are being displayed.
    this._pointSearch = null;

    // The closest trace point to the mouse. May be {} if no traces are
    // displayed or the mouse hasn't moved over the canvas yet. Has the form:
    //   {
    //     x: x,
    //     y: y,
    //     name: String, // name of trace
    //   }
    this._hoverPt = {
      x: -1,
      y: -1,
      name: '',
    };

    // The location of the crosshair in canvas coordinates. Of the form:
    //   {
    //     x: x,
    //     y: y,
    //     shift: Boolean,
    //   }
    //
    // The value of shift is true of the shift key is being pressed while the
    // mouse moves.
    // }
    this._crosshair = {
      x: -1,
      y: -1,
      shift: false,
    };

    // All the info we need about the summary area.
    this._summary = {
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

    // All the info we need about the details area.
    this._detail = {
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

    // The total number of points we are displaying. Used to decide whether or
    // not to update the details traces when zooming.
    this._numPoints = 0;

    // A task to rebuild the k-d search tree used for finding the closest point
    // to the mouse. The actual value is a window.setTimer timerId or zero if no
    // task is scheduled.
    //
    // See https://jakearchibald.com/2015/tasks-microtasks-queues-and-schedules/
    // for details on tasks vs microtasks.
    this._recalcSearchTask = 0;

    // A task to do the actual re-draw work of a zoom. The actual value is a
    // window.setTimer timerId or zero if no task is scheduled.
    //
    // See https://jakearchibald.com/2015/tasks-microtasks-queues-and-schedules/
    // for details on tasks vs microtasks.
    this._zoomTask = 0;

    // A formatter that prints numbers nicely, such as adding commas. Used when
    // display the hover text.
    this._numberFormatter = new Intl.NumberFormat();

    this._upgradeProperty('width');
    this._upgradeProperty('height');
    this._upgradeProperty('bands');
    this._upgradeProperty('xbar');
    this._upgradeProperty('hightlight');
    this._upgradeProperty('zoom');

    this._updateScaledMeasurements();
  }

  /**
   * Update all the things that look like constants, but are really dependent on
   * window.devicePixelRatio or the current CSS styling.
   */
  _updateScaledMeasurements() {
    // The height of the summary area.
    if (this.summary) {
      this.SUMMARY_HEIGHT = 50 * this._scale; // px
    } else {
      this.SUMMARY_HEIGHT = 0;
    }

    this.SUMMARY_BAR_WIDTH = 2 * this._scale; // px

    this.DETAIL_BAR_WIDTH = 3 * this._scale; // px

    this.SUMMARY_HIGHLIGHT_LINE_WIDTH = 3 * this._scale;

    // The radius of points in the details area.
    this.DETAIL_RADIUS = 3 * this._scale; // px

    // The radius of points in the summary area.
    this.SUMMARY_RADIUS = 2 * this._scale; // px

    // The margin around the details and summary areas.
    this.MARGIN = 32 * this._scale; // px

    this.LEFT_MARGIN = 2 * this.MARGIN; // px

    this.Y_AXIS_TICK_LENGTH = this.MARGIN / 4; // px

    this.LABEL_FONT_SIZE = 14 * this._scale; // px

    this.LABEL_MARGIN = 6 * this._scale; // px

    this.LABEL_FONT = `${this.LABEL_FONT_SIZE}px Roboto,Helvetica,Arial,Bitstream Vera Sans,sans-serif`;

    this.ZOOM_BAR_LINE_WIDTH = 3 * this._scale; // px

    this.HOVER_LINE_WIDTH = 1 * this._scale; // px

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

  connectedCallback() {
    super.connectedCallback();

    this.render();

    // We need to dynamically resize the canvas elements since they don't do
    // that themselves.
    const resizeObserver = new LocalResizeObserver((entries) => {
      entries.forEach((entry) => {
        this.width = entry.contentRect.width;
      });
    });
    resizeObserver.observe(this);

    this.addEventListener('mousemove', (e) => {
      // Do as little as possible here. The _raf() function will periodically
      // check if the mouse has moved and trigger the appropriate redraws.
      this._mouseMoveRaw = {
        clientX: e.clientX,
        clientY: e.clientY,
        shiftKey: e.shiftKey,
      };
    });

    this.addEventListener('mousedown', (e) => {
      const pt = this._eventToCanvasPt(e);
      // If you click in the summary area then begin zooming via drag.
      if (inRect(pt, this._summary.rect!)) {
        const zx = this._summary.range.x.invert(pt.x);
        this._inZoomDrag = true;
        this._zoomBegin = zx;
        this.zoom = [zx, zx + 0.01]; // Add a smidge to the second zx to avoid a degenerate detail plot.
      }
    });

    this.addEventListener('mouseup', () => {
      if (this._inZoomDrag) {
        this._dispatchZoomEvent();
      }
      this._inZoomDrag = false;
    });

    this.addEventListener('mouseleave', () => {
      if (this._inZoomDrag) {
        this._dispatchZoomEvent();
      }
      this._inZoomDrag = false;
    });

    this.addEventListener('click', (e) => {
      const pt = this._eventToCanvasPt(e);
      if (!inRect(pt, this._detail.rect)) {
        return;
      }
      if (!this._pointSearch) {
        return;
      }
      const closest = this._pointSearch.nearest(pt);
      const detail = {
        x: closest.sx,
        y: closest.sy,
        name: closest.name,
      };
      this.dispatchEvent(
        new CustomEvent<PlotSimpleSkTraceEventDetails>('trace_selected', {
          detail: detail,
          bubbles: true,
        })
      );
    });

    // If the user toggles the theme to/from darkmode then redraw.
    document.addEventListener('theme-chooser-toggle', () => {
      this.render();
    });

    window.requestAnimationFrame(this._raf.bind(this));
  }

  _dispatchZoomEvent() {
    if (!this._zoom) {
      return;
    }
    let beginIndex = Math.floor(this._zoom[0] - 0.1);
    if (beginIndex < 0) {
      beginIndex = 0;
    }
    let endIndex = Math.ceil(this._zoom[1] + 0.1);
    if (endIndex > this._labels.length - 1) {
      endIndex = this._labels.length - 1;
    }
    const detail = {
      xBegin: this._labels[beginIndex],
      xEnd: this._labels[endIndex],
    };
    this.dispatchEvent(
      new CustomEvent<PlotSimpleSkZoomEventDetails>('zoom', {
        detail: detail,
        bubbles: true,
      })
    );
  }

  /**
   * Convert mouse event coordinates to a canvas point.
   *
   * @param {Object} e - A mouse event or an object that has the coords stored
   * in clientX and clientY.
   */
  _eventToCanvasPt(e: MouseMoveRaw) {
    const clientRect = this._ctx!.canvas.getBoundingClientRect();
    return {
      x: (e.clientX - clientRect.left) * this._scale,
      y: (e.clientY - clientRect.top) * this._scale,
    };
  }

  // Handles requestAnimationFrame callbacks.
  _raf() {
    // Always queue up our next raf first.
    window.requestAnimationFrame(() => this._raf());

    // Bail out early if the mouse hasn't moved.
    if (this._mouseMoveRaw === null) {
      return;
    }
    if (this._inZoomDrag === false) {
      const pt = this._eventToCanvasPt(this._mouseMoveRaw);

      // Update _hoverPt if needed.
      if (this._pointSearch) {
        const closest = this._pointSearch.nearest(pt);
        const detail = {
          x: closest.sx,
          y: closest.sy,
          name: closest.name,
        };
        if (detail.x !== this._hoverPt.x || detail.y !== this._hoverPt.y) {
          this._hoverPt = detail;
          this.dispatchEvent(
            new CustomEvent<PlotSimpleSkTraceEventDetails>('trace_focused', {
              detail: detail,
              bubbles: true,
            })
          );
        }
      }

      // Update crosshair.
      if (this._mouseMoveRaw.shiftKey && this._pointSearch) {
        this._crosshair = {
          x: pt.x,
          y: pt.y,
          shift: false,
        };
        clampToRect(this._crosshair, this._detail.rect);
      } else {
        this._crosshair = {
          x: this._detail.range.x(this._hoverPt.x),
          y: this._detail.range.y(this._hoverPt.y),
          shift: true,
        };
      }
      this._drawOverlayCanvas();
      this._mouseMoveRaw = null;
    } else {
      // We are zooming.
      const pt = this._eventToCanvasPt(this._mouseMoveRaw);
      clampToRect(pt, this._summary.rect);

      // x in source coordinates.
      const sx = this._summary.range.x.invert(pt.x);

      // Set zoom, always making sure we go from lowest to highest.
      let zoom: ZoomRange = [this._zoomBegin, sx];
      if (this._zoomBegin > sx) {
        zoom = [sx, this._zoomBegin];
      }
      this.zoom = zoom;
    }
  }

  /**
   * This is a super simple hash (h = h * 31 + x_i) currently used
   * for things like assigning colors to graphs based on trace ids. It
   * shouldn't be used for anything more serious than that.
   *
   * @param {String} s - A string to hash.
   * @return {Number} A 32 bit hash for the given string.
   */
  _hashString(s: string) {
    let hash = 0;
    for (let i = s.length - 1; i >= 0; i--) {
      hash = (hash << 5) - hash + s.charCodeAt(i);
      hash |= 0;
    }
    return Math.abs(hash);
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
  addLines(lines: { [key: string]: number[] }, labels: Date[]) {
    const keys = Object.keys(lines);
    if (keys.length === 0) {
      return;
    }
    const startedEmpty = this._zoom === null && this._lineData.length === 0;
    if (labels) {
      this._labels = labels;
    }

    // Convert into the format we will eventually expect.
    keys.forEach((key) => {
      // You can't encode NaN in JSON, so convert sentinel values to NaN here so
      // that dsArray functions will operate correctly.
      lines[key].forEach((x, i) => {
        if (x === MISSING_DATA_SENTINEL) {
          lines[key][i] = NaN;
        }
      });
      const values = lines[key];
      this._lineData.push({
        name: key,
        values: values,
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
    if (startedEmpty && this._lineData.length > 0) {
      this._zoom = [0, this._lineData[0].values.length - 1];
    }

    this._updateCount();
    this._updateScaleDomains();
    this._recalcSummaryPaths();
    this._recalcDetailPaths();
    this._drawTracesCanvas();
  }

  _updateCount() {
    this._numPoints = 0;
    if (this._lineData.length > 0) {
      this._numPoints = this._lineData.length * this._lineData[0].values.length;
    }
  }

  // Rebuilds our cache of Path2D objects we use for quick rendering.
  _recalcSummaryPaths() {
    this._lineData.forEach((line) => {
      // Need to pass in the x and y ranges, and the dot radius.
      if (line.name.startsWith(SPECIAL)) {
        line.color = this.LABEL_COLOR;
      } else {
        line.color = COLORS[(this._hashString(line.name) % 8) + 1];
      }

      const summaryBuilder = new PathBuilder(
        this._summary.range.x,
        this._summary.range.y,
        this.SUMMARY_RADIUS
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
    this._recalcXAxis(this._summary, this._labels, 0);
  }

  // Rebuilds our cache of Path2D objects we use for quick rendering.
  _recalcDetailPaths() {
    const domain = this._detail.range.x.domain();
    domain[0] = Math.floor(domain[0] - 0.1);
    domain[1] = Math.ceil(domain[1] + 0.1);
    this._lineData.forEach((line) => {
      // Need to pass in the x and y ranges, and the dot radius.
      if (line.name.startsWith(SPECIAL)) {
        line.color = this.LABEL_COLOR;
      } else {
        line.color = COLORS[(this._hashString(line.name) % 8) + 1];
      }

      const detailBuilder = new PathBuilder(
        this._detail.range.x,
        this._detail.range.y,
        this.DETAIL_RADIUS
      );

      line.values.forEach((y, x) => {
        if (Number.isNaN(y)) {
          return;
        }
        if (x < domain[0] || x > domain[1]) {
          return;
        }
        detailBuilder.add(x, y);
      });
      line.detail = detailBuilder.paths();
    });

    // Build detail x-axis.
    const detailDomain = this._detail.range.x.domain();
    const labelOffset = Math.ceil(detailDomain[0]);
    const detailLabels = this._labels.slice(
      Math.ceil(detailDomain[0]),
      Math.floor(detailDomain[1] + 1)
    );
    this._recalcXAxis(this._detail, detailLabels, labelOffset);

    // Build detail y-axis.
    this._recalcYAxis(this._detail);
    this._recalcSearch();
  }

  // Recalculates the y-axis info.
  _recalcYAxis(area: DetailArea) {
    const yAxisPath = new Path2D();
    const thin_x = Math.floor(this._detail.rect.x) + 0.5; // Make sure we get a thin line. https://developer.mozilla.org/en-US/docs/Web/API/Canvas_API/Tutorial/Applying_styles_and_colors#A_lineWidth_example
    yAxisPath.moveTo(thin_x, this._detail.rect.y);
    yAxisPath.lineTo(thin_x, this._detail.rect.y + this._detail.rect.height);
    area.yaxis.labels = [];
    area.range.y.ticks(NUM_Y_TICKS).forEach((t) => {
      const label = {
        x: 0,
        y: Math.floor(area.range.y(t)) + 0.5,
        text: `${this._numberFormatter.format(t)}`,
      };
      area.yaxis.labels.push(label);
      yAxisPath.moveTo(thin_x, label.y);
      yAxisPath.lineTo(thin_x - this.Y_AXIS_TICK_LENGTH, label.y);
    });
    area.yaxis.path = yAxisPath;
  }

  // Recalculates the x-axis info.
  _recalcXAxis(area: Area, labels: Date[], labelOffset: number) {
    const xAxisPath = new Path2D();
    const thin_y = Math.floor(area.rect.y) + 0.5; // Make sure we get a thin line.
    xAxisPath.moveTo(area.rect.x + 0.5, thin_y);
    xAxisPath.lineTo(area.rect.x + 0.5 + area.rect.width, thin_y);
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
  _recalcSearch() {
    if (this._recalcSearchTask) {
      return;
    }
    this._recalcSearchTask = window.setTimeout(() => this._recalcSearchImpl());
  }

  _recalcSearchImpl() {
    if (this._zoomTask) {
      // If there is a pending zoom task then let that complete first since zooming
      // invalidates the search tree and it needs to be built again.
      this._recalcSearchTask = window.setTimeout(() =>
        this._recalcSearchImpl()
      );
      return;
    }
    const domain = this._detail.range.x.domain();
    domain[0] = Math.floor(domain[0] - 0.1);
    domain[1] = Math.ceil(domain[1] + 0.1);
    const searchBuilder = new SearchBuilder(
      this._detail.range.x,
      this._detail.range.y
    );
    this._lineData.forEach((line) => {
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
    this._pointSearch = searchBuilder.kdTree();
    this._recalcSearchTask = 0;
  }

  // Updates all of our d3Scale domains.
  _updateScaleDomains() {
    let domainEnd = 1;
    if (this._lineData && this._lineData.length) {
      domainEnd = this._lineData[0].values.length - 1;
    }
    if (this._zoom) {
      this._detail.range.x = this._detail.range.x.domain(this._zoom);
    } else {
      this._detail.range.x = this._detail.range.x.domain([0, domainEnd]);
    }

    this._summary.range.x = this._summary.range.x.domain([0, domainEnd]);

    const domain = [
      d3Array.min(this._lineData, (line) => d3Array.min(line.values))!,
      d3Array.max(this._lineData, (line) => d3Array.max(line.values))!,
    ];

    this._detail.range.y = this._detail.range.y.domain(domain).nice();

    this._summary.range.y = this._summary.range.y.domain(domain);
  }

  // Updates all of our d3Scale ranges. Also updates detail and summary rects.
  _updateScaleRanges() {
    const width = this._ctx!.canvas.width;
    const height = this._ctx!.canvas.height;

    this._summary.range.x = this._summary.range.x.range([
      this.LEFT_MARGIN,
      width - this.MARGIN,
    ]);

    this._summary.range.y = this._summary.range.y.range([
      this.SUMMARY_HEIGHT + this.MARGIN,
      this.MARGIN,
    ]);

    this._detail.range.x = this._detail.range.x.range([
      this.LEFT_MARGIN,
      width - this.MARGIN,
    ]);

    this._detail.range.y = this._detail.range.y.range([
      height - this.MARGIN,
      this.SUMMARY_HEIGHT + 2 * this.MARGIN,
    ]);

    this._summary.rect = {
      x: this.LEFT_MARGIN,
      y: this.MARGIN,
      width: width - this.MARGIN - this.LEFT_MARGIN,
      height: this.SUMMARY_HEIGHT,
    };

    this._detail.rect = {
      x: this.LEFT_MARGIN,
      y: this.SUMMARY_HEIGHT + 2 * this.MARGIN,
      width: width - this.MARGIN - this.LEFT_MARGIN,
      height: height - this.SUMMARY_HEIGHT - 3 * this.MARGIN,
    };
  }

  // Draw the contents of the overlay canvas.
  _drawOverlayCanvas() {
    // Always start by clearing the overlay.
    const width = this._overlayCtx!.canvas.width;
    const height = this._overlayCtx!.canvas.height;
    const ctx = this._overlayCtx!;

    ctx.clearRect(0, 0, width, height);

    if (this.summary) {
      // First clip to the summary region.
      ctx.save();
      {
        // Block to scope save/restore.
        clipToRect(ctx, this._summary.rect);

        // Draw the xbar.
        this._drawXBar(ctx, this._summary, this.SUMMARY_BAR_WIDTH);

        // Draw the bands.
        this._drawBands(ctx, this._summary, this.SUMMARY_BAR_WIDTH);

        // Draw the zoom on the summary.
        if (this._zoom !== null) {
          ctx.lineWidth = this.ZOOM_BAR_LINE_WIDTH;
          ctx.strokeStyle = this.LABEL_COLOR;

          // Draw left bar.
          const leftx = this._summary.range.x(this._zoom[0]);
          ctx.beginPath();
          ctx.moveTo(leftx, this._summary.rect.y);
          ctx.lineTo(leftx, this._summary.rect.y + this._summary.rect.height);

          // Draw right bar.
          const rightx = this._summary.range.x(this._zoom[1]);
          ctx.moveTo(rightx, this._summary.rect.y);
          ctx.lineTo(rightx, this._summary.rect.y + this._summary.rect.height);
          ctx.stroke();

          // Draw gray boxes.
          ctx.fillStyle = ZOOM_RECT_COLOR;
          ctx.rect(
            this._summary.rect.x,
            this._summary.rect.y,
            leftx - this._summary.rect.x,
            this._summary.rect.height
          );
          ctx.rect(
            rightx,
            this._summary.rect.y,
            this._summary.rect.x + this._summary.rect.width - rightx,
            this._summary.rect.height
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
      clipToRect(ctx, this._detail.rect);

      // Draw the xbar.
      this._drawXBar(ctx, this._detail, this.DETAIL_BAR_WIDTH);

      // Draw the bands.
      this._drawBands(ctx, this._detail, this.DETAIL_BAR_WIDTH);

      // Draw highlighted lines.
      this._lineData.forEach((line) => {
        if (!(line.name in this._highlighted)) {
          return;
        }
        ctx.strokeStyle = line.color;
        ctx.fillStyle = this.LABEL_BACKGROUND;
        ctx.lineWidth = this.SUMMARY_HIGHLIGHT_LINE_WIDTH;

        ctx.stroke(line.detail.linePath!);
        ctx.fill(line.detail.dotsPath!);
        ctx.stroke(line.detail.dotsPath!);
      });

      // Find the line currently hovered over.
      let line = null;
      for (let i = 0; i < this._lineData.length; i++) {
        if (this._lineData[i].name === this._hoverPt.name) {
          line = this._lineData[i];
          break;
        }
      }
      if (line !== null) {
        // Draw the hovered line and dots in a different color.
        ctx.strokeStyle = HOVER_COLOR;
        ctx.fillStyle = HOVER_COLOR;
        ctx.lineWidth = this.HOVER_LINE_WIDTH;

        // Just draw the dots, not the line.
        ctx.fill(line.detail.dotsPath!);
        ctx.stroke(line.detail.dotsPath!);
      }

      if (!this._inZoomDrag) {
        // Draw the crosshairs.
        ctx.strokeStyle = this.CROSSHAIR_COLOR;
        ctx.lineWidth = AXIS_LINE_WIDTH;
        ctx.beginPath();
        const thin_x = Math.floor(this._crosshair.x) + 0.5; // Make sure we get a thin line.
        const thin_y = Math.floor(this._crosshair.y) + 0.5; // Make sure we get a thin line.
        ctx.moveTo(this._detail.rect.x, thin_y);
        ctx.lineTo(this._detail.rect.x + this._detail.rect.width, thin_y);
        ctx.moveTo(thin_x, this._detail.rect.y);
        ctx.lineTo(thin_x, this._detail.rect.y + this._detail.rect.height);
        ctx.stroke();

        // Y label at crosshair if shift is pressed.
        if (this._crosshair.shift) {
          // Draw the label offset from the crosshair.
          ctx.font = this.LABEL_FONT;
          ctx.textBaseline = 'bottom';
          const label = this._numberFormatter.format(this._hoverPt.y);
          let x = this._crosshair.x + this.MARGIN;
          let y = this._crosshair.y - this.MARGIN;

          // First draw a white backdrop.
          ctx.fillStyle = this.LABEL_BACKGROUND;
          const meas = ctx.measureText(label);
          const labelHeight = this.LABEL_FONT_SIZE + 2 * this.LABEL_MARGIN;
          const labelWidth = meas.width + this.LABEL_MARGIN * 2;

          // Bump the text to different quadrants so it is always visible.
          if (y < this._detail.rect.y + this._detail.rect.height / 2) {
            y = this._crosshair.y + this.MARGIN;
          }
          if (x > this._detail.rect.x + this._detail.rect.width / 2) {
            x = x - labelWidth - 2 * this.MARGIN;
          }

          ctx.beginPath();
          ctx.rect(
            x - this.LABEL_MARGIN,
            y + this.LABEL_MARGIN,
            labelWidth,
            -labelHeight
          );
          ctx.fill();
          ctx.strokeStyle = this.LABEL_COLOR;
          ctx.beginPath();
          ctx.rect(
            x - this.LABEL_MARGIN,
            y + this.LABEL_MARGIN,
            labelWidth,
            -labelHeight
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

  // Draw the xbar in the given area with the given width.
  _drawXBar(ctx: CanvasRenderingContext2D, area: Area, width: number) {
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
  _drawBands(ctx: CanvasRenderingContext2D, area: Area, width: number) {
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
  _drawTracesCanvas() {
    const width = this._ctx!.canvas.width;
    const height = this._ctx!.canvas.height;
    const ctx = this._ctx!;

    if (this._inZoomDrag) {
      ctx.clearRect(
        this._detail.rect.x - this.MARGIN,
        this._detail.rect.y - this.MARGIN,
        this._detail.rect.width + 2 * this.MARGIN,
        this._detail.rect.height + 2 * this.MARGIN
      );
    } else {
      ctx.clearRect(0, 0, width, height);
    }
    ctx.fillStyle = this.LABEL_BACKGROUND;

    // Draw the detail.
    ctx.save();
    {
      // Block to scope save/restore.
      clipToRect(ctx, this._detail.rect);
      this._drawXAxis(ctx, this._detail);
      ctx.fillStyle = this.LABEL_BACKGROUND;

      this._lineData.forEach((line) => {
        ctx.strokeStyle = line.color;
        ctx.lineWidth = DETAIL_LINE_WIDTH;
        ctx.stroke(line.detail.linePath!);
        ctx.fill(line.detail.dotsPath!);
        ctx.stroke(line.detail.dotsPath!);
      });
    }
    ctx.restore();
    this._drawXAxis(ctx, this._detail);

    if (!this._inZoomDrag && this.summary) {
      // Draw the summary.
      ctx.save();
      {
        // Block to scope save/restore.
        clipToRect(ctx, this._summary.rect);
        this._lineData.forEach((line) => {
          ctx.fillStyle = this.LABEL_BACKGROUND;
          ctx.strokeStyle = line.color;
          ctx.lineWidth = SUMMARY_LINE_WIDTH;
          ctx.stroke(line.summary.linePath!);
          ctx.fill(line.summary.dotsPath!);
          ctx.stroke(line.summary.dotsPath!);
        });
      }
      ctx.restore();
      this._drawXAxis(ctx, this._summary);
    }
    // Draw y-Axes.
    this._drawYAxis(ctx, this._detail);

    this._drawOverlayCanvas();
  }

  // Draw a y-axis using the given context in the given area.
  _drawYAxis(ctx: CanvasRenderingContext2D, area: DetailArea) {
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
  _drawXAxis(ctx: CanvasRenderingContext2D, area: Area) {
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
   * Delete all the lines whose ids are in 'ids' from being plotted.
   *
   * @param {Array<string>} ids - The trace ids to remove.
   */
  deleteLines(ids: string[]) {
    this._lineData = this._lineData.filter(
      (line) => ids.indexOf(line.name) === -1
    );

    const onlySpecialLinesRemaining = this._lineData.every((line) =>
      line.name.startsWith(SPECIAL)
    );
    if (onlySpecialLinesRemaining) {
      this.removeAll();
    } else {
      this._updateCount();
      this._updateScaleDomains();
      this._recalcSummaryPaths();
      this._recalcDetailPaths();
      this._drawTracesCanvas();
    }
  }

  /**
   * Remove all lines from plot.
   */
  removeAll() {
    this._lineData = [];
    this._labels = [];
    this._hoverPt = {
      x: -1,
      y: -1,
      name: '',
    };
    this._pointSearch = null;
    this._crosshair = {
      x: -1,
      y: -1,
      shift: false,
    };
    this._mouseMoveRaw = null;
    this._highlighted = {};
    this._xbar = -1;
    this._zoom = null;
    this._inZoomDrag = false;
    this._numPoints = 0;
    this._drawTracesCanvas();
  }

  /**
   * @prop {Array} ids - An array of trace ids to highlight. Set to [] to remove
   * all highlighting.
   */
  get highlight() {
    return Object.keys(this._highlighted);
  }

  set highlight(ids) {
    this._highlighted = {};
    ids.forEach((name) => {
      this._highlighted[name] = true;
    });
    this._drawOverlayCanvas();
  }

  /**
   * @prop {Number} xbar - Location to put a vertical marking bar on the graph.
   * Can be set to -1 to not display any bar.
   */
  set xbar(value) {
    this._xbar = value;
    this._drawOverlayCanvas();
  }

  get xbar() {
    return this._xbar;
  }

  /**
   * @prop {Array} bands - A list of x source offsets to place vertical markers.
   *   into labels. Can be set to [] to remove all bands.
   */
  get bands() {
    return this._bands;
  }

  set bands(bands) {
    if (!bands) {
      this._bands = [];
    } else {
      this._bands = bands;
    }
    this._drawOverlayCanvas();
  }

  /** @prop zoom {Array} The zoom range, an array of two values in source x
   * units. Can be set to null to have no zoom.
   */
  get zoom() {
    return this._zoom;
  }

  set zoom(range) {
    this._zoom = range;
    if (this._zoomTask) {
      return;
    }
    this._zoomTask = window.setTimeout(() => this._zoomImpl());
  }

  _zoomImpl() {
    this._updateScaleDomains();
    this._recalcDetailPaths();
    this._drawTracesCanvas();
    this._zoomTask = 0;
  }

  static get observedAttributes() {
    return ['width', 'height', 'summary'];
  }

  /** @prop width {string} Mirrors the width attribute. */
  get width() {
    return +(this.getAttribute('width') || '0');
  }

  set width(val) {
    this.setAttribute('width', val.toString());
  }

  /** @prop height {string} Mirrors the height attribute. */
  get height() {
    return +(this.getAttribute('height') || '0');
  }

  set height(val) {
    this.setAttribute('height', val.toString());
  }

  /** @prop summary {string} Mirrors the summary attribute. */
  get summary() {
    return this.hasAttribute('summary');
  }

  set summary(val) {
    if (val) {
      this.setAttribute('summary', val.toString());
    } else {
      this.removeAttribute('summary');
    }
  }

  attributeChangedCallback(_: string, oldValue: string, newValue: string) {
    if (oldValue !== newValue) {
      this.render();
    }
  }

  // Call this when the width or height attrs have changed.
  render() {
    this._render();
    const canvas = this.querySelector<HTMLCanvasElement>('canvas.traces')!;
    const overlayCanvas = this.querySelector<HTMLCanvasElement>(
      'canvas.overlay'
    )!;
    if (canvas) {
      this._ctx = canvas.getContext('2d');
      this._overlayCtx = overlayCanvas.getContext('2d');
      this._scale = window.devicePixelRatio;
      this._updateScaledMeasurements();
      this._updateScaleRanges();
      this._recalcDetailPaths();
      this._recalcSummaryPaths();
      this._drawTracesCanvas();
    }
  }
}

define('plot-simple-sk', PlotSimpleSk);
