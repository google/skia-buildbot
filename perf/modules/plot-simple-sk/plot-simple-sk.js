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
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
 *    |   |                                            |   |
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
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import * as d3Scale from 'd3-scale'
import * as d3Array from 'd3-array'
import { kdTree } from './kd.js'

/**
 * @constant {string} - Prefix for trace ids that are not real traces.
 */
const SPECIAL = 'special';

// The height of the summary area.
const SUMMARY_HEIGHT = 50;

// The radius of points in the details area.
const RADIUS = 4;

// The radius of points in the summary area.
const SUMMARY_RADIUS = 2;

// The margin around the details and summary areas.
const MARGIN = 20; // px

const LABEL_FONT_SIZE = 16; // px

const LABEL_MARGIN = 6; // px

const LABEL_FONT = `${LABEL_FONT_SIZE}px Roboto,Helvetica,Arial,Bitstream Vera Sans,sans-serif`;

/**
 * @constant {Array} - Colors used for traces.
 */
const colors = [
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

// Builds the Path2D objects that describe the trace
// and the dots for a given set of scales.
class PathBuilder {
  constructor(xRange, yRange, radius) {
    this.xRange = xRange;
    this.yRange = yRange;
    this.radius = radius;
    this.linePath = new Path2D();
    this.dotsPath = new Path2D();
  }

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

  paths() {
    return {
      _linePath: this.linePath,
      _dotsPath: this.dotsPath,
    }
  }
}


// Builds a kdTree for searcing for nearest points to
// the mouse.
class SearchBuilder {
  constructor() {
    this.points = [];
  }

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

  kdTree() {
    const distance = (a, b) => {
      return (a.x - b.x) * (a.x - b.x) + (a.y - b.y) * (a.y - b.y);
    }

    return new kdTree(this.points, distance, ["x", "y"]);
  }
}

function inRect(rect, pt) {
  return pt.x >= rect.x
    && pt.x <= rect.x + rect.width
    && pt.y >= rect.y
    && pt.y <= rect.y + rect.height;
}

const template = (ele) => html`
  <canvas class=traces width=${ele.width * window.devicePixelRatio} height=${ele.height * window.devicePixelRatio}
    style="width: ${ele.width}px; height: ${ele.height}px;"
  ></canvas>
  <canvas class=overlay width=${ele.width * window.devicePixelRatio} height=${ele.height * window.devicePixelRatio}
    style="width: ${ele.width}px; height: ${ele.height}px;"
  ></canvas>
`;

define('plot-simple-sk', class extends ElementSk {
  constructor() {
    super(template);

    // The location of the XBar. See setXBar().
    this._xbar = -1;

    // The locations of the background bands. See setBanding().
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
    // displayed or the mouse hasn't moved over the canvas yet.
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

    this._summary = {
      rect: null,
      range: {
        x: d3Scale.scaleLinear(),
        y: d3Scale.scaleLinear(),
      }
    }

    this._detail = {
      rect: null,
      range: {
        x: d3Scale.scaleLinear(),
        y: d3Scale.scaleLinear(),
      },
    }
  }

  connectedCallback() {
    super.connectedCallback();

    this.render();

    // Add mousemovewatcher which gets mousemove
    // events and a sub-rect of the canvas to
    // watch. Takes a callback that is called
    // when there is a move in the rect and supplies
    // the x and y coords to the callback.
    this.addEventListener("mousemove", e => {
      this._mouseMoveRaw = {
        clientX: e.clientX,
        clientY: e.clientY,
        shiftKey: e.shiftKey,
      };
    });

    this.addEventListener("mousedown", e => {
      const pt = this._eventToCanvasPt(e)

      if (inRect(this._summary.rect, pt)) {
        const zx = this._summary.range.x.invert(pt.x);
        this._inZoomDrag = true;
        this._zoomBegin = zx;
        this.zoom([zx, zx]);
      }
    });

    this.addEventListener("mouseup", e => {
      if (this._inZoomDrag) {
        this.dispatchEvent(new CustomEvent('zoom', { detail: this._zoom, bubbles: true }));
      }
      this._inZoomDrag = false;
    });

    this.addEventListener("mouseleave", e => {
      if (this._inZoomDrag) {
        this.dispatchEvent(new CustomEvent('zoom', { detail: this._zoom, bubbles: true }));
      }
      this._inZoomDrag = false;
    });

    this.addEventListener("click", e => {
      const pt = this._eventToCanvasPt(e);
      if (!inRect(this._detail.rect, pt)) {
        return
      }
      const sx = this._detail.range.x.invert(pt.x);
      const sy = this._detail.range.y.invert(pt.y);
      const closest = this._pointSearch.nearest({ x: sx, y: sy }, 1)[0][0];
      this.dispatchEvent(new CustomEvent('trace_selected', { detail: closest, bubbles: true }));
    });

    window.requestAnimationFrame(this._raf.bind(this));
  }

  // e is a mouse event or an object that has the coords stored
  // in clientX and clientY.
  _eventToCanvasPt(e) {
    const clientRect = this._ctx.canvas.getBoundingClientRect();
    return {
      x: (e.clientX - clientRect.left) * this._scale,
      y: (e.clientY - clientRect.top) * this._scale,
    }
  }


  _raf() {
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
        const closest = this._pointSearch.nearest({ x: sx, y: sy }, 1)[0][0];
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
      }
      this._drawOverlay();
      this._mouseMoveRaw = null;
    } else {
      const pt = this._eventToCanvasPt(this._mouseMoveRaw);

      // Clamp x to the summary rect;
      if (pt.x < this._summary.rect.x) {
        pt.x = this._summary.rect.x;
      } else if (pt.x > this._summary.rect.x + this._summary.rect.width) {
        pt.x = this._summary.rect.x + this._summary.rect.width;
      }

      // x in source coordinates.
      const sx = this._summary.range.x.invert(pt.x);

      let zoom = [this._zoomBegin, sx];
      if (this._zoomBegin > sx) {
        zoom = [sx, this._zoomBegin];
      }
      this.zoom(zoom);
    }

    window.requestAnimationFrame(this._raf.bind(this));
  }

  /**
   * This is a super simple hash (h = h * 31 + x_i) currently used
   * for things like assigning colors to graphs based on trace ids. It
   * shouldn't be used for anything more serious than that.
   *
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
   * @param {Object} lines - A map from trace id to arrays of [x, y] pairs.
   * @param {Array=} labels - An array of Date's that represent the x values of
   *   data to plot.
   *
   * TODO(jcgregorio) Switch lines to be a map to an Array of y values
   *   since that's what chart.js expects.
   *
   * @example
   *
   *     let lines = [
   *       {
   *         name: foo,
   *         values: [3.7, 3.8, 3.9],
   *       },
   *       {
   *         name: bar,
   *         values: [2.5, 4.2, 3.9],
   *       }
   *     ]
   *     plot.addLines(lines);
   */
  addLines(lines) {
    const startedEmpty = this._lineData.length === 0;

    // Convert into the format we will eventually expect.
    Object.keys(lines).forEach(key => {
      // You can't encode NaN in JSON, so convert sentinel values to NaN here.
      lines[key].forEach((x, i) => {
        if (x === 1e32) {
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

    if (startedEmpty) {
      this._zoom = [0, this._lineData[0].values.length - 1];
    }

    this._updateScaleDomains();
    this._recalcPaths();
    this._recalcSearch();
    this._plot();
  }

  _recalcPaths() {
    this._lineData.forEach(line => {
      // Need to pass in the x and y ranges, and the dot radius.
      if (line.name.startsWith(SPECIAL)) {
        line._color = "#000";
      } else {
        line._color = colors[(this._hashString(line.name) % 8) + 1];
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
  }

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

    // Use this._detail.range.y.domain()[0] and [1]
    // along with _xDetailRange.domain() to build
    // a fast lookup for the closest point.

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
    {
      ctx.beginPath();
      ctx.rect(this._summary.rect.x, this._summary.rect.y, this._summary.rect.width, this._summary.rect.height);
      ctx.clip();

      // Draw the zoom on the summary.
      if (this._zoom !== null) {
        ctx.lineWidth = 5;
        ctx.strokeStyle = "#000";

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
        ctx.fillStyle = "#0003"
        ctx.rect(this._summary.rect.x, this._summary.rect.y, leftx - this._summary.rect.x, this._summary.rect.height);
        ctx.rect(rightx, this._summary.rect.y, this._summary.rect.x + this._summary.rect.width - rightx, this._summary.rect.height);

        ctx.fill();
      }
    }
    ctx.restore();

    // Now clip to the detail region.
    ctx.save();
    ctx.beginPath();
    ctx.rect(this._detail.rect.x, this._detail.rect.y, this._detail.rect.width, this._detail.rect.height);
    ctx.clip();

    // Draw the xbar.
    if (this._xbar !== -1) {
      ctx.lineWidth = 3;
      ctx.strokeStyle = "#900";
      const bx = this._detail.range.x(this._xbar);
      ctx.beginPath();
      ctx.moveTo(bx, this._detail.rect.y);
      ctx.lineTo(bx, this._detail.rect.y + this._detail.rect.height);
      ctx.stroke();
    }

    // Draw the bands.
    this._bands.forEach(band => {
      ctx.lineWidth = 3;
      ctx.strokeStyle = "#888";
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
      ctx.fillStyle = "#fff";
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
      ctx.strokeStyle = "#999";
      ctx.fillStyle = "#999";
      ctx.lineWidth = 1;

      ctx.stroke(line.detail._linePath);
      ctx.fill(line.detail._dotsPath);
      ctx.stroke(line.detail._dotsPath);
    }

    if (!this._inZoomDrag) {
      // Draw the crosshairs.
      ctx.strokeStyle = "#900";
      ctx.fillStyle = "#900";
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
        ctx.fillStyle = "#fff"
        const meas = ctx.measureText(label);
        const height = (LABEL_FONT_SIZE + 2 * LABEL_MARGIN) * this._scale;
        const width = meas.width + LABEL_MARGIN * 2 * this._scale;
        ctx.beginPath();
        ctx.rect(x - LABEL_MARGIN * this._scale, y + LABEL_MARGIN * this._scale, width, -height);
        ctx.fill();

        // Now draw text on top.
        ctx.fillStyle = "#000"
        ctx.fillText(label, x, y);
      }
    }

    ctx.restore();
  }

  _plot() {
    const width = this._ctx.canvas.width;
    const height = this._ctx.canvas.height;
    const ctx = this._ctx;

    ctx.clearRect(0, 0, width, height);
    ctx.fillStyle = "#fff";

    // Draw the detail.

    // First clip to the detail rect.
    ctx.save();
    ctx.beginPath();
    ctx.rect(this._detail.rect.x, this._detail.rect.y, this._detail.rect.width, this._detail.rect.height);
    ctx.clip();

    this._lineData.forEach(line => {
      ctx.strokeStyle = line._color;
      ctx.stroke(line.detail._linePath);
      ctx.fill(line.detail._dotsPath);
      ctx.stroke(line.detail._dotsPath);
    })
    ctx.restore();

    // Draw the summary.

    // First clip to the detail rect.
    ctx.save();
    ctx.beginPath();
    ctx.rect(this._summary.rect.x, this._summary.rect.y, this._summary.rect.width, this._summary.rect.height);
    ctx.clip();

    this._lineData.forEach(line => {
      ctx.strokeStyle = line._color;
      ctx.stroke(line.summary._linePath);
      ctx.fill(line.summary._dotsPath);
      ctx.stroke(line.summary._dotsPath);
    })
    ctx.restore();

    // Draw y-Axes.
    const yDetailAxis = new Path2D();
    ctx.strokeStyle = "#000";
    ctx.fillStyle = "#000";
    ctx.textBaseline = 'middle';
    this._detail.range.y.ticks().forEach(t => {
      const ty = this._detail.range.y(t);
      yDetailAxis.moveTo(3 * MARGIN / 4, ty);
      yDetailAxis.lineTo(MARGIN, ty);
      ctx.fillText('' + t, 0, ty, MARGIN / 2);
    });
    yDetailAxis.moveTo(this._detail.rect.x, this._detail.rect.y);
    yDetailAxis.lineTo(this._detail.rect.x, this._detail.rect.y + this._detail.rect.height);
    ctx.stroke(yDetailAxis);

    this._drawOverlay();
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
   * Highlight one or more traces.
   *
   * @param {Array} ids - An array of trace ids.
   */
  setHighlight(ids) {
    this._highlighted = {};
    ids.forEach(name => {
      this._highlighted[name] = true;
    });
    this._drawOverlay();
  }

  /**
   * Returns the trace ids of all highlighted traces.
   *
   * @return {Array} Trace ids.
   */
  highlighted() {
    return this._highlighted.keys();
  }

  /**
   * Clears all highlighting from all traces.
   */
  clearHighlight() {
    this._highlighted = {};
    this._drawOverlay();
  }

  /**
   * Turns on a vertical bar at the given position.
   *
   * @param {Number} x - The offset into the labels where the bar
   *   should be positioned.
   */
  setXBar(x) {
    this._xbar = x;
    this._drawOverlay();
  }

  /**
   * Removes the x-bar from being displayed.
   *
   * @param {Number} x - The offset into the labels where the bar
   *   should be removed from.
   */
  clearXBar(x) {
    this._xbar = -1;
    this._drawOverlay();
  }

  /**
   * Sets the banding over ranges of labels.
   *
   * @param {Array} bands - A list of [x1, x2] offsets
   *   into labels.
   *
   * @example
   *
   *     let bands = [
   *       [0.0, 0.1],
   *       [0.5, 1.2],
   *     ];
   *     plot.setBanding(bands);
   */
  setBanding(bands) {
    // Just draw these are vertical dashed lines in the overlay.
    this._bands = [];
    bands.forEach(band => {
      this._bands.push(band[0]);
      this._bands.push(band[1]);
    })
    this._drawOverlay();
  }

  /**
   * Resets the zoom to default.
   */
  resetAxes() {
    this._zoom = null;
    this._updateScaleDomains();
    this._recalcPaths();
    this._plot();
  }

  zoom(range) {
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
    const canvas = this.querySelector("canvas.traces");
    const overlayCanvas = this.querySelector("canvas.overlay");
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

