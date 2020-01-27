/**
 * @module modules/plot-simple-sk
 * @description <h2><code>plot-simple-sk</code></h2>
 *
 *  A custom element for plotting x,y graphs.
 *
 *  The canvas is broken into two areas, the summary and the details. The
 *  summary is always SUMMARY_HEIGHT pixels high. Also note that we use
 *  window.devicePixelRatio to decide the actual number of pixels to use, and
 *  then use CSS to squash the canvas back down to the desired size.
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
 *    | M |                                            | M |
 *    | A |                                            | A |
 *    | R |                                            | R |
 *    | G |                                            | G |
 *    | I |                                            | I |
 *    | N |                                            | N |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   +--------------------------------------------+   |
 *    |                                                    |
 *    |                   MARGIN                           |
 *    |                                                    |
 *    +----------------------------------------------------+
 *
 * To keep rendering quick the traces will be written into Path2D objects
 * to be used for quick rendering.
 *
 * We also use a k-d Tree for quick lookup for clicking and mouse movement
 * over the traces.
 *
 * @evt trace_selected - Event produced when the user clicks on a line.
 *     The e.detail contains the id of the line and the index of the
 *     point in the line closest to the mouse, and the [x, y] value
 *     of the point in 'pt'.
 *
 *     <pre>
 *       e.detail = {
 *         id: 'id of trace',
 *         index: 3,
 *         pt: [2, 34.5],
 *       }
 *     </pre>
 *
 * @evt trace_focused - Event produced when the user moves the mouse close
 *     to a line. The e.detail contains the id of the line and the index of the
 *     point in the line closest to the mouse.
 *     <pre>
 *       e.detail = {
 *         id: 'id of trace',
 *         index: 3,
 *         pt: [2, 34.5],
 *       }
 *     </pre>
 *
 * @evt zoom - Event produced when the user has zoomed into a region
 *      by dragging.
 *
 * @attr width - The width of the element in px.
 * @attr height - The height of the element in px.
 *
 */
import { define } from 'elements-sk/define'
import { html } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import * as d3Scale from 'd3-scale'
import * as d3Array from 'd3-array'
import { kdTree } from './kd'
import { ticks } from './ticks'

//  Prefix for trace ids that are not real traces.
const SPECIAL = 'special';

// The height of the summary area.
const SUMMARY_HEIGHT = 50;

// The radius of points in the details area.
const RADIUS = 4;

// The radius of points in the summary area.
const SUMMARY_RADIUS = 2;

// The margin around the details and summary areas.
const MARGIN = 32; // px

const LABEL_FONT_SIZE = 14; // px

const LABEL_MARGIN = 6; // px

const LABEL_FONT = `${LABEL_FONT_SIZE}px Roboto,Helvetica,Arial,Bitstream Vera Sans,sans-serif`;

const LABEL_COLOR = '#000';

const LABEL_BACKGROUND = '#fff';

const CROSSHAIR_COLOR = '#f00';

const SPECIAL_COLOR = '#000';

const HOVER_COLOR = '#555';

const XBAR_COLOR = '#900';

const BAND_COLOR = '#888';

const DOT_FILL_COLOR = '#fff';

const ZOOM_BAR_COLOR = '#000';

const ZOOM_RECT_COLOR = '#0003'; // Note the alpha value.

const MISSING_DATA_SENTINEL = 1e32;

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

/** @class Builds the Path2D objects that describe the trace and the dots for a given
*   set of scales.
*/
class PathBuilder {
  constructor(xRange, yRange, radius) {
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
  add(x, y) {
    const cx = this.xRange(x);
    const cy = this.yRange(y);

    if (x == 0) {
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
  paths() {
    return {
      _linePath: this.linePath,
      _dotsPath: this.dotsPath,
    }
  }
}


/**
 * @class Builds a kdTree for searcing for nearest points to the mouse.
 */
class SearchBuilder {
  constructor() {
    this.points = [];
  }

  /**
   * Add a point to the kdTree.
   *
   * @param {Number} x - X coordinate in source coordinates.
   * @param {Number} y - Y coordinate in source coordinates.
   * @param {String} name - The trace name.
   */
  add(x, y, name) {
    if (name.startsWith(SPECIAL)) {
      return
    }
    this.points.push(
      {
        x: x,
        y: y,
        name: name,
      }
    )
  }

  /**
   * Returns a kdTree that contians all the points being plotted.
   *
   * @returns {kdTree}
   */
  kdTree() {
    const distance = (a, b) => {
      return (a.x - b.x) * (a.x - b.x) + (a.y - b.y) * (a.y - b.y);
    }

    return new kdTree(this.points, distance, ['x', 'y']);
  }
}

// Returns true if pt is in rect.
function inRect(pt, rect) {
  return pt.x >= rect.x
    && pt.x < rect.x + rect.width
    && pt.y >= rect.y
    && pt.y < rect.y + rect.height;
}

// Restricts pt to rect.
function clampToRect(pt, rect) {
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
function clipToRect(ctx, rect) {
  ctx.beginPath();
  ctx.rect(rect.x, rect.y, rect.width, rect.height);
  ctx.clip();
}

const template = (ele) => html`
  <canvas class=traces width=${ele.width * window.devicePixelRatio} height=${ele.height * window.devicePixelRatio}
    style='transform-origin: 0 0; transform: scale(${1 / window.devicePixelRatio});'
  ></canvas>
  <canvas class=overlay width=${ele.width * window.devicePixelRatio} height=${ele.height * window.devicePixelRatio}
    style='transform-origin: 0 0; transform: scale(${1 / window.devicePixelRatio});'
  ></canvas>`;

define('plot-simple-sk', class extends ElementSk {
  constructor() {
    super(template);

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
    //       _linePath: Path2D,
    //       _dotsPath: Path2D,
    //     },
    //     summary: {
    //       _linePath: Path2D,
    //       _dotsPath: Path2D,
    //     },
    //   }
    this._lineData = [];

    // An arra of Date()'s the same length as the values in _lineData.
    this._labels = [];

    // The current zoom, either null or an array of two values in source x
    // coordinates, e.g. [1, 12].
    this._zoom = null;

    // True if we are currently drag zooming, i.e. the mouse is pressed and
    // moving over the summary.
    this._inZoomDrag = false;

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
    this._hoverPt = {};

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
    this._crosshair = {};

    // All the info we need about the summary area.
    this._summary = {
      rect: null,
      axis: {
        path: new Path2D(), // Path2D.
        labels: [], // The labels and locations to draw them. {x, y, text}.
      },
      range: {
        x: d3Scale.scaleLinear(),
        y: d3Scale.scaleLinear(),
      }
    }

    // All the info we need about the details area.
    this._detail = {
      rect: null,
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
    }
  }

  connectedCallback() {
    super.connectedCallback();

    this.render();

    this.addEventListener('mousemove', e => {
      // Do as little as possible here. The _raf() function will periodically
      // check if the mouse has moved and trigger the appropriate redraws.
      this._mouseMoveRaw = {
        clientX: e.clientX,
        clientY: e.clientY,
        shiftKey: e.shiftKey,
      };
    });

    this.addEventListener('mousedown', e => {
      const pt = this._eventToCanvasPt(e)
      // If you click in the summary area then begin zooming via drag.
      if (inRect(pt, this._summary.rect)) {
        const zx = this._summary.range.x.invert(pt.x);
        this._inZoomDrag = true;
        this._zoomBegin = zx;
        this.zoom = [zx, zx + 0.01]; // Add a smidge to the second zx to avoid a degenerate detail plot.
      }
    });

    this.addEventListener('mouseup', e => {
      if (this._inZoomDrag) {
        this.dispatchEvent(new CustomEvent('zoom', { detail: this._zoom, bubbles: true }));
      }
      this._inZoomDrag = false;
    });

    this.addEventListener('mouseleave', e => {
      if (this._inZoomDrag) {
        this.dispatchEvent(new CustomEvent('zoom', { detail: this._zoom, bubbles: true }));
      }
      this._inZoomDrag = false;
    });

    this.addEventListener('click', e => {
      const pt = this._eventToCanvasPt(e);
      if (!inRect(pt, this._detail.rect)) {
        return
      }
      const sx = this._detail.range.x.invert(pt.x);
      const sy = this._detail.range.y.invert(pt.y);
      const closest = this._pointSearch.nearest({ x: sx, y: sy }, 1)[0][0];
      this.dispatchEvent(new CustomEvent('trace_selected', { detail: closest, bubbles: true }));
    });

    window.requestAnimationFrame(this._raf.bind(this));
  }

  /**
   * Convert mouse event coordiates to a canvas point.
   *
   * @param {Object} e - A mouse event or an object that has the coords stored
   * in clientX and clientY.
  */
  _eventToCanvasPt(e) {
    const clientRect = this._ctx.canvas.getBoundingClientRect();
    return {
      x: (e.clientX - clientRect.left) * this._scale,
      y: (e.clientY - clientRect.top) * this._scale,
    }
  }

  // Handles requestAnimationFrame callbacks.
  _raf() {
    // Bail out early if the mouse hasn't moved.
    if (this._mouseMoveRaw === null) {
      window.requestAnimationFrame(this._raf.bind(this));
      return
    }
    if (this._inZoomDrag == false) {
      const pt = this._eventToCanvasPt(this._mouseMoveRaw);

      // x,y in source coordinates.
      const sx = this._detail.range.x.invert(pt.x);
      const sy = this._detail.range.y.invert(pt.y);

      // Update _hoverPt if needed.
      if (this._pointSearch) {
        const closest = this._pointSearch.nearest({ x: sx, y: sy }, 1);
        if (closest.x !== this._hoverPt.x || closest.y !== this._hoverPt.y) {
          this._hoverPt = closest;
          this.dispatchEvent(new CustomEvent('trace_focused', { detail: closest, bubbles: true }));
        }
      }

      // Update crosshair.
      if (this._mouseMoveRaw.shiftKey && this._pointSearch) {
        this._crosshair = {
          x: this._detail.range.x(this._hoverPt.x),
          y: this._detail.range.y(this._hoverPt.y),
          shift: true,
        }
      } else {
        this._crosshair = {
          x: pt.x,
          y: pt.y,
          shift: false,
        }
        clampToRect(this._crosshair, this._detail.rect);
      }
      this._drawOverlay();
      this._mouseMoveRaw = null;
    } else {
      const pt = this._eventToCanvasPt(this._mouseMoveRaw);
      clampToRect(pt, this._summary.rect);

      // x in source coordinates.
      const sx = this._summary.range.x.invert(pt.x);

      // Set zoom, always making sure we go from lowest to highest.
      let zoom = [this._zoomBegin, sx];
      if (this._zoomBegin > sx) {
        zoom = [sx, this._zoomBegin];
      }
      this.zoom = zoom;
    }

    window.requestAnimationFrame(this._raf.bind(this));
  }

  /**
   * This is a super simple hash (h = h * 31 + x_i) currently used
   * for things like assigning colors to graphs based on trace ids. It
   * shouldn't be used for anything more serious than that.
   *
   * @param {String} s - A string to hash.
   * @return {Number} A 32 bit hash for the given string.
   */
  _hashString(s) {
    let hash = 0;
    for (let i = s.length - 1; i >= 0; i--) {
      hash = ((hash << 5) - hash) + s.charCodeAt(i);
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
  addLines(lines, labels) {
    const startedEmpty = this._lineData.length === 0;
    if (labels) {
      this._labels = labels;
    }

    // Convert into the format we will eventually expect.
    Object.keys(lines).forEach(key => {
      // You can't encode NaN in JSON, so convert sentinel values to NaN here so
      // that dsArray functions will operate correctly.
      lines[key].forEach((x, i) => {
        if (x === MISSING_DATA_SENTINEL) {
          lines[key][i] = NaN;
        }
      })
      this._lineData.push({
        name: key,
        values: lines[key],
        detail: {},
        summary: {}
      })
    })

    // Set the zoom if we just added data for the first time.
    if (startedEmpty) {
      this._zoom = [0, this._lineData[0].values.length - 1];
    }

    this._updateScaleDomains();
    this._recalcPaths();
    this._recalcSearch();
    this._plot();
  }

  // Rebuild all our cache of Path2D objects we use for quick rendering.
  _recalcPaths() {
    this._lineData.forEach(line => {
      // Need to pass in the x and y ranges, and the dot radius.
      if (line.name.startsWith(SPECIAL)) {
        line._color = SPECIAL_COLOR;
      } else {
        line._color = COLORS[(this._hashString(line.name) % 8) + 1];
      }

      const detailBuilder = new PathBuilder(this._detail.range.x, this._detail.range.y, RADIUS);
      const summaryBuilder = new PathBuilder(this._summary.range.x, this._summary.range.y, SUMMARY_RADIUS);

      line.values.forEach((y, x) => {
        if (isNaN(y)) {
          return
        }
        detailBuilder.add(x, y);
        summaryBuilder.add(x, y);
      });
      line.detail = detailBuilder.paths();
      line.summary = summaryBuilder.paths();
    })

    // Build summary tick marks.
    this._recalcXAxis(this._summary, this._labels, 0);

    // Build detail tick marks.
    const detailDomain = this._detail.range.x.domain();
    const labelOffset = Math.ceil(detailDomain[0]);
    const detailLabels = this._labels.slice(Math.ceil(detailDomain[0]), Math.floor(detailDomain[1] + 1));
    this._recalcXAxis(this._detail, detailLabels, labelOffset);
    this._recalcYAxis(this._detail);
  }

  // Recalculates the y-axis info.
  _recalcYAxis(area) {
    const yAxisPath = new Path2D();
    yAxisPath.moveTo(this._detail.rect.x, this._detail.rect.y);
    yAxisPath.lineTo(this._detail.rect.x, this._detail.rect.y + this._detail.rect.height);
    area.yaxis.labels = [];
    area.range.y.ticks().forEach(t => {
      const label = {
        x: 0,
        y: area.range.y(t),
        text: '' + t,
      }
      area.yaxis.labels.push(label);
      yAxisPath.moveTo(2 * MARGIN / 3, label.y);
      yAxisPath.lineTo(MARGIN, label.y);
    });
    area.yaxis.path = yAxisPath;
  }

  // Recalculates the x-axis info.
  _recalcXAxis(area, labels, labelOffset) {
    let xAxisPath = new Path2D();
    xAxisPath.moveTo(area.rect.x, area.rect.y);
    xAxisPath.lineTo(area.rect.x + area.rect.width, area.rect.y);
    area.axis.labels = [];
    ticks(labels).forEach(tick => {
      const label = {
        x: area.range.x(tick.x + labelOffset),
        y: area.rect.y - MARGIN / 2,
        text: tick.text,
      };
      area.axis.labels.push(label);
      xAxisPath.moveTo(label.x, area.rect.y);
      xAxisPath.lineTo(label.x, area.rect.y - MARGIN / 2);
    });
    area.axis.path = xAxisPath;
  }

  // Rebuilds the kdTree we use to look up closest points.
  _recalcSearch() {
    const searchBuilder = new SearchBuilder();

    this._lineData.forEach(line => {
      line.values.forEach((y, x) => {
        if (isNaN(y)) {
          return
        }
        searchBuilder.add(x, y, line.name);
      });
    })
    this._pointSearch = searchBuilder.kdTree();
  }

  _updateScaleDomains() {
    if (this._zoom) {
      this._detail.range.x = this._detail.range.x
        .domain(this._zoom);
    } else {
      this._detail.range.x = this._detail.range.x
        .domain([0, this._lineData[0].values.length - 1]);
    }

    this._summary.range.x = this._summary.range.x
      .domain([0, this._lineData[0].values.length - 1]);

    const domain = [
      d3Array.min(this._lineData, line => d3Array.min(line.values)),
      d3Array.max(this._lineData, line => d3Array.max(line.values))
    ];

    this._detail.range.y = this._detail.range.y
      .domain(domain)
      .nice();

    this._summary.range.y = this._summary.range.y
      .domain(domain);
  }

  _updateScaleRanges() {
    const width = this._ctx.canvas.width;
    const height = this._ctx.canvas.height;

    this._summary.range.x = this._summary.range.x
      .range([
        MARGIN,
        width - MARGIN
      ]);

    this._summary.range.y = this._summary.range.y
      .range([
        SUMMARY_HEIGHT + MARGIN,
        MARGIN
      ])

    this._detail.range.x = this._detail.range.x
      .range([
        MARGIN,
        width - MARGIN
      ]);

    this._detail.range.y = this._detail.range.y
      .range([
        height - MARGIN,
        SUMMARY_HEIGHT + 2 * MARGIN
      ])

    this._summary.rect = {
      x: MARGIN,
      y: MARGIN,
      width: width - 2 * MARGIN,
      height: SUMMARY_HEIGHT,
    };

    this._detail.rect = {
      x: MARGIN,
      y: SUMMARY_HEIGHT + 2 * MARGIN,
      width: width - 2 * MARGIN,
      height: height - SUMMARY_HEIGHT - 3 * MARGIN,
    }
  }

  _drawOverlay() {
    // Always start by clearing the overlay.
    const width = this._overlayCtx.canvas.width;
    const height = this._overlayCtx.canvas.height;
    const ctx = this._overlayCtx;

    ctx.clearRect(0, 0, width, height);

    // First clip to the summary region.
    ctx.save();
    { // Block to scope save/restore.

      clipToRect(ctx, this._summary.rect);

      // Draw the zoom on the summary.
      if (this._zoom !== null) {
        ctx.lineWidth = 5;
        ctx.strokeStyle = ZOOM_BAR_COLOR;

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
        ctx.rect(this._summary.rect.x, this._summary.rect.y, leftx - this._summary.rect.x, this._summary.rect.height);
        ctx.rect(rightx, this._summary.rect.y, this._summary.rect.x + this._summary.rect.width - rightx, this._summary.rect.height);

        ctx.fill();
      }
    }
    ctx.restore();

    // Now clip to the detail region.
    ctx.save();
    { // Block to scope save/restore.
      clipToRect(ctx, this._detail.rect);

      // Draw the xbar.
      if (this._xbar !== -1) {
        ctx.lineWidth = 3;
        ctx.strokeStyle = XBAR_COLOR;
        const bx = this._detail.range.x(this._xbar);
        ctx.beginPath();
        ctx.moveTo(bx, this._detail.rect.y);
        ctx.lineTo(bx, this._detail.rect.y + this._detail.rect.height);
        ctx.stroke();
      }

      // Draw the bands.
      this._bands.forEach(band => {
        ctx.lineWidth = 3;
        ctx.strokeStyle = BAND_COLOR;
        ctx.setLineDash([5, 5]);
        const bx = this._detail.range.x(band);
        ctx.beginPath();
        ctx.moveTo(bx, this._detail.rect.y);
        ctx.lineTo(bx, this._detail.rect.y + this._detail.rect.height);
        ctx.stroke();
        ctx.setLineDash([]);
      });

      // Draw highlighted lines.
      this._lineData.forEach(line => {
        if (!this._highlighted.hasOwnProperty(line.name)) {
          return
        }
        ctx.strokeStyle = line._color;
        ctx.fillStyle = DOT_FILL_COLOR;
        ctx.lineWidth = 3;

        ctx.stroke(line.detail._linePath);
        ctx.fill(line.detail._dotsPath);
        ctx.stroke(line.detail._dotsPath);
      })

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
        ctx.lineWidth = 1;

        ctx.stroke(line.detail._linePath);
        ctx.fill(line.detail._dotsPath);
        ctx.stroke(line.detail._dotsPath);
      }

      if (!this._inZoomDrag) {
        // Draw the crosshairs.
        ctx.strokeStyle = CROSSHAIR_COLOR;
        ctx.beginPath();
        ctx.moveTo(this._detail.rect.x, this._crosshair.y);
        ctx.lineTo(this._detail.rect.x + this._detail.rect.width, this._crosshair.y);

        ctx.moveTo(this._crosshair.x, this._detail.rect.y);
        ctx.lineTo(this._crosshair.x, this._detail.rect.y + this._detail.rect.height);
        ctx.stroke();

        // Y label at crosshair if shift is pressed.
        if (this._crosshair.shift) {
          // Draw the label offset from the crosshair.
          ctx.font = LABEL_FONT;
          ctx.textBaseline = 'bottom';
          const label = '' + this._hoverPt.y;
          const x = this._crosshair.x + MARGIN
          const y = this._crosshair.y - MARGIN;

          // First draw a white backdrop.
          ctx.fillStyle = LABEL_BACKGROUND;
          const meas = ctx.measureText(label);
          const height = (LABEL_FONT_SIZE + 2 * LABEL_MARGIN) * this._scale;
          const width = meas.width + LABEL_MARGIN * 2 * this._scale;
          ctx.beginPath();
          ctx.rect(x - LABEL_MARGIN * this._scale, y + LABEL_MARGIN * this._scale, width, -height);
          ctx.fill();

          // Now draw text on top.
          ctx.fillStyle = LABEL_COLOR;
          ctx.fillText(label, x, y);
        }
      }
    }
    ctx.restore();
  }

  _plot() {
    const width = this._ctx.canvas.width;
    const height = this._ctx.canvas.height;
    const ctx = this._ctx;

    ctx.clearRect(0, 0, width, height);
    ctx.fillStyle = DOT_FILL_COLOR;

    // Draw the detail.
    ctx.save();
    { // Block to scope save/restore.
      clipToRect(ctx, this._detail.rect);
      this._drawXAxis(ctx, this._detail);
      this._lineData.forEach(line => {
        ctx.strokeStyle = line._color;
        ctx.fillStyle = DOT_FILL_COLOR;
        ctx.stroke(line.detail._linePath);
        ctx.fill(line.detail._dotsPath);
        ctx.stroke(line.detail._dotsPath);
      })
    }
    ctx.restore();
    this._drawXAxis(ctx, this._detail);

    // Draw the summary.
    ctx.save();
    { // Block to scope save/restore.
      clipToRect(ctx, this._summary.rect);
      this._lineData.forEach(line => {
        ctx.fillStyle = DOT_FILL_COLOR;
        ctx.strokeStyle = line._color;
        ctx.stroke(line.summary._linePath);
        ctx.fill(line.summary._dotsPath);
        ctx.stroke(line.summary._dotsPath);
      })
    }
    ctx.restore();
    this._drawXAxis(ctx, this._summary);

    // Draw y-Axes.
    this._drawYAxis(ctx, this._detail);

    this._drawOverlay();
  }

  _drawYAxis(ctx, area) {
    ctx.strokeStyle = LABEL_COLOR;
    ctx.fillStyle = LABEL_COLOR;
    ctx.font = LABEL_FONT;
    ctx.textBaseline = 'middle';
    ctx.stroke(area.yaxis.path);
    area.yaxis.labels.forEach(label => {
      ctx.fillText(label.text, label.x, label.y, 2 * MARGIN / 3);
    });
  }

  _drawXAxis(ctx, area) {
    ctx.strokeStyle = LABEL_COLOR;
    ctx.fillStyle = LABEL_COLOR;
    ctx.font = LABEL_FONT;
    ctx.textBaseline = 'middle';
    ctx.stroke(area.axis.path);
    area.axis.labels.forEach(label => {
      ctx.fillText(label.text, label.x + 2, label.y);
    });
  }

  /**
   * Delete a line from being plotted.
   *
   * @param {string} id - The trace id.
   */
  deleteLine(id) {
    for (let i = 0; i < this._lineData.length; i++) {
      if (this._lineData[i].name === id) {
        this._lineData.splice(i, 1);
      }
    }
    this._updateScaleDomains();
    this._recalcPaths();
    this._recalcSearch();
    this._plot();
  }

  /**
   * Remove all lines from plot.
   */
  removeAll() {
    this._lineData = [];
    this._hoverPt = {};
    this._pointSearch = null;
    this._crosshair = {};
    this._mouseMoveRaw = null;
    this._highlighted = {};
    this._xbar = -1;
    this._zoom = null;
    this._inZoomDrag = false;
    this._plot();
  }

  /**
   * @prop {Array} ids - An array of trace ids to highlight. Set to [] to remove
   * all highlighting.
   */
  get highlight() { return this._highlighted.keys(); }
  set highlight(ids) {
    this._highlighted = {};
    ids.forEach(name => {
      this._highlighted[name] = true;
    });
    this._drawOverlay();
  }

  /**
   * @prop {Number} xbar - Location to put a vertical marking bar on the graph.
   * Can be set to -1 to not display any bar.
   */
  set xbar(value) {
    this._xbar = value;
    this._drawOverlay();
  }
  get xbar() { return this._xbar; }

  /**
   * @prop {Array} bands - A list of x source offsets to place vertical markers.
   *   into labels. Can be set to [] to remove all bands.
   */
  get bands() { return this._bands; }
  set bands(bands) {
    // For now translate the legacy format of bands.
    // Just draw these are vertical dashed lines in the overlay.
    this._bands = [];
    bands.forEach(band => {
      this._bands.push(band[0]);
      this._bands.push(band[1]);
    })
    this._drawOverlay();
  }

  /** @prop zoom {Array} The zoom range, an array of two values in source x
   * units. Can be set to null to have no zoom.
   */
  get zoom() { return this._zoom; }
  set zoom(range) {
    this._zoom = range;
    this._updateScaleDomains();
    this._recalcPaths();
    this._plot();
  }

  static get observedAttributes() {
    return ['width', 'height'];
  }

  /** @prop width {string} Mirrors the width attribute. */
  get width() { return this.getAttribute('width'); }
  set width(val) { this.setAttribute('width', val); }

  /** @prop height {string} Mirrors the height attribute. */
  get height() { return this.getAttribute('height'); }
  set height(val) { this.setAttribute('height', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    if (oldValue !== newValue) {
      this.render();
    }
  }

  // Call this when the width or height attrs have changed.
  render() {
    this._render();
    const canvas = this.querySelector('canvas.traces');
    const overlayCanvas = this.querySelector('canvas.overlay');
    if (canvas) {
      this._ctx = canvas.getContext('2d');
      this._overlayCtx = overlayCanvas.getContext('2d');
      this._scale = window.devicePixelRatio;
      this._updateScaleRanges();
      this._recalcPaths();
      this._plot();
    }
  }

});