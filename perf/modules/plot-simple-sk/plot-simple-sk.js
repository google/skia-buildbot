/**
 * @module modules/plot-simple-sk
 * @description <h2><code>plot-simple-sk</code></h2>
 *
 *  A custom element for plotting x,y graphs.
 *
 * @evt trace_selected - Event produced when the user clicks on a line.
 *     The e.detail contains the id of the line and the index of the
 *     point in the line closest to the mouse, and the [x, y] value
 *     of the point in 'pt'.
 *
 *     <pre>
 *       e.detail = {
 *          id: 'id of trace',
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
import * as d3 from 'd3'
import { kdTree } from './kd.js'

/**
 * @constant {string} - Prefix for trace ids that are not real traces.
 */
const SPECIAL = 'special';

const SUMMARY_HEIGHT = 50;

const RADIUS = 4;
const SUMMARY_RADIUS = 2;

const MARGIN = 20; // px

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

    this._highlighted = {};

    this._lineData = [];
    this._zoom = null;

    this._ctx = null;
    this._overlayCtx = null;
    this._scale = 1.0; // The window.devicePixelRatio.

    this._mouseMoveRaw = null;
    this._pointSearch = null;
    this._hoverPt = {};
    this._crosshair = {};

    // Should replace below with
    // this.summary.rect and .range.x and .range.y

    this._summaryRect = null;
    this._detailRect = null;

    this._xDetailRange = d3.scaleLinear();
    this._yDetailRange = d3.scaleLinear();
    this._xSummaryRange = d3.scaleLinear();
    this._ySummaryRange = d3.scaleLinear();

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
        x: e.clientX,
        y: e.clientY,
        shift: e.shiftKey,
      };
    });

    window.requestAnimationFrame(this._raf.bind(this));
  }

  _raf() {
    if (this._mouseMoveRaw === null) {
      window.requestAnimationFrame(this._raf.bind(this));
      return
    }
    const clientRect = this._ctx.canvas.getBoundingClientRect();
    // x,y on canvas.
    const x = (this._mouseMoveRaw.x - clientRect.left) * this._scale;
    const y = (this._mouseMoveRaw.y - clientRect.top) * this._scale;
    // x,y in source coordinates.
    const sx = this._xDetailRange.invert(x);
    const sy = this._yDetailRange.invert(y);

    let needsRedraw = false;

    // Update _hoverPt if needed.
    if (this._pointSearch) {
      const closest = this._pointSearch.nearest({ x: sx, y: sy }, 1)[0][0];
      if (closest.x !== this._hoverPt.x || closest.y !== this._hoverPt.y) {
        this._hoverPt = closest;
        needsRedraw = true;
      }
    }

    // Update _crosshair if needed.
    if (this._crosshair.x != x || this._crosshair.y != y) {
      if (this._mouseMoveRaw.shift && this._pointSearch) {
        this._crosshair = {
          x: this._xDetailRange(this._hoverPt.x),
          y: this._yDetailRange(this._hoverPt.y),
          shift: true,
        }
      } else {
        this._crosshair = {
          x: x,
          y: y,
          shift: false,
        }
      }
      needsRedraw = true;
    }

    if (needsRedraw) {
      this._drawOverlay();
    }
    this._mouseMoveRaw = null;
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
   *     plot.addLines(lines, labels);
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
    this._plot();
  }

  _recalcPaths() {
    const searchBuilder = new SearchBuilder();

    this._lineData.forEach(line => {
      // Need to pass in the x and y ranges, and the dot radius.
      line._color = colors[(this._hashString(line.name) % 8) + 1];

      const detailBuilder = new PathBuilder(this._xDetailRange, this._yDetailRange, RADIUS);
      const summaryBuilder = new PathBuilder(this._xSummaryRange, this._ySummaryRange, SUMMARY_RADIUS);

      line.values.forEach((y, x) => {
        if (isNaN(y)) {
          return
        }
        searchBuilder.add(x, y, line.name);
        detailBuilder.add(x, y);
        summaryBuilder.add(x, y);
      });
      line.detail = detailBuilder.paths();
      line.summary = summaryBuilder.paths();
    })
    this._pointSearch = searchBuilder.kdTree();
  }

  _updateScaleDomains() {
    if (this._zoom) {
      this._xDetailRange = this._xDetailRange
        .domain(this._zoom);
    } else {
      this._xDetailRange = this._xDetailRange
        .domain([0, this._lineData[0].values.length]);
    }

    this._xSummaryRange = this._xSummaryRange
      .domain([0, this._lineData[0].values.length]);

    const domain = [
      d3.min(this._lineData, line => d3.min(line.values)),
      d3.max(this._lineData, line => d3.max(line.values))
    ];

    this._yDetailRange = this._yDetailRange
      .domain(domain)
      .nice();

    // Use this._yDetailRange.domain()[0] and [1]
    // along with _xDetailRange.domain() to build
    // a fast lookup for the closest point.

    this._ySummaryRange = this._ySummaryRange
      .domain(domain);
  }

  _updateScaleRanges() {
    const width = this._ctx.canvas.width;
    const height = this._ctx.canvas.height;

    // What proportion of the canvas do we use
    // for summary vs detail?
    // And how do we apply the margin?

    // The summary is always SUMMARY_HEIGHT pixels.

    this._xSummaryRange = this._xSummaryRange
      .range([
        MARGIN,
        width - MARGIN
      ]);

    this._ySummaryRange = this._ySummaryRange
      .range([
        SUMMARY_HEIGHT + MARGIN,
        MARGIN
      ])

    this._xDetailRange = this._xDetailRange
      .range([
        MARGIN,
        width - MARGIN
      ]);

    this._yDetailRange = this._yDetailRange
      .range([
        height - MARGIN,
        SUMMARY_HEIGHT + 2 * MARGIN
      ])

    this._summaryRect = {
      x: MARGIN,
      y: MARGIN,
      width: width - 2 * MARGIN,
      height: SUMMARY_HEIGHT,
    };

    this._detailRect = {
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
    this._overlayCtx.clearRect(0, 0, width, height);

    // First clip to the summary region.
    this._overlayCtx.save();
    {
      this._overlayCtx.beginPath();
      this._overlayCtx.rect(this._summaryRect.x, this._summaryRect.y, this._summaryRect.width, this._summaryRect.height);
      this._overlayCtx.clip();

      // Draw the zoom on the summary.
      if (this._zoom !== null) {
        this._overlayCtx.lineWidth = 5;
        this._overlayCtx.strokeStyle = "#000";

        // Draw left bar.
        const leftx = this._xSummaryRange(this._zoom[0]);
        this._overlayCtx.beginPath();
        this._overlayCtx.moveTo(leftx, this._summaryRect.y);
        this._overlayCtx.lineTo(leftx, this._summaryRect.y + this._summaryRect.height);

        // Draw right bar.
        const rightx = this._xSummaryRange(this._zoom[1]);
        this._overlayCtx.moveTo(rightx, this._summaryRect.y);
        this._overlayCtx.lineTo(rightx, this._summaryRect.y + this._summaryRect.height);
        this._overlayCtx.stroke();

        // Draw gray boxes.
        this._overlayCtx.fillStyle = "#0003"
        this._overlayCtx.rect(this._summaryRect.x, this._summaryRect.y, leftx - this._summaryRect.x, this._summaryRect.height);
        this._overlayCtx.rect(rightx, this._summaryRect.y, this._summaryRect.x + this._summaryRect.width - rightx, this._summaryRect.height);

        this._overlayCtx.fill();
      }
    }
    this._overlayCtx.restore();

    // Now clip to the detail region.
    this._overlayCtx.save();
    this._overlayCtx.beginPath();
    this._overlayCtx.rect(this._detailRect.x, this._detailRect.y, this._detailRect.width, this._detailRect.height);
    this._overlayCtx.clip();

    // Draw the xbar.
    if (this._xbar !== -1) {
      this._overlayCtx.lineWidth = 3;
      this._overlayCtx.strokeStyle = "#900";
      const bx = this._xDetailRange(this._xbar);
      this._overlayCtx.beginPath();
      this._overlayCtx.moveTo(bx, this._detailRect.y);
      this._overlayCtx.lineTo(bx, this._detailRect.y + this._detailRect.height);
      this._overlayCtx.stroke();
    }

    // Draw the bands.
    this._bands.forEach(band => {
      this._overlayCtx.lineWidth = 3;
      this._overlayCtx.strokeStyle = "#ddd";
      // set dashed?
      const bx = this._xDetailRange(band);
      this._overlayCtx.beginPath();
      this._overlayCtx.moveTo(bx, this._detailRect.y);
      this._overlayCtx.lineTo(bx, this._detailRect.y + this._detailRect.height);
      this._overlayCtx.stroke();
    });

    // Draw highlighted lines.
    this._lineData.forEach(line => {
      if (!this._highlighted.hasOwnProperty(line.name)) {
        return
      }
      this._overlayCtx.strokeStyle = line._color;
      this._overlayCtx.fillStyle = "#fff";
      this._overlayCtx.lineWidth = 3;

      this._overlayCtx.stroke(line.detail._linePath);
      this._overlayCtx.fill(line.detail._dotsPath);
      this._overlayCtx.stroke(line.detail._dotsPath);
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
      this._overlayCtx.strokeStyle = "#999";
      this._overlayCtx.fillStyle = "#999";
      this._overlayCtx.lineWidth = 1;

      this._overlayCtx.stroke(line.detail._linePath);
      this._overlayCtx.fill(line.detail._dotsPath);
      this._overlayCtx.stroke(line.detail._dotsPath);
    }

    // Draw the crosshairs.
    this._overlayCtx.strokeStyle = "#900";
    this._overlayCtx.fillStyle = "#900";
    this._overlayCtx.beginPath();
    this._overlayCtx.moveTo(this._detailRect.x, this._crosshair.y);
    this._overlayCtx.lineTo(this._detailRect.x + this._detailRect.width, this._crosshair.y);

    this._overlayCtx.moveTo(this._crosshair.x, this._detailRect.y);
    this._overlayCtx.lineTo(this._crosshair.x, this._detailRect.y + this._detailRect.height);
    this._overlayCtx.stroke();

    // Y label at crosshair if shift is pressed.
    if (this._crosshair.shift) {
      this._overlayCtx.font = 'bold 20px sans-serif';
      // First draw a white backdrop.
      this._overlayCtx.fillStyle = "#fff"
      this._overlayCtx.fillText('' + this._hoverPt.y, this._crosshair.x + MARGIN + 2, this._crosshair.y - MARGIN - 2);
      this._overlayCtx.fillText('' + this._hoverPt.y, this._crosshair.x + MARGIN - 2, this._crosshair.y - MARGIN + 2);
      this._overlayCtx.fillStyle = "#000"
      this._overlayCtx.fillText('' + this._hoverPt.y, this._crosshair.x + MARGIN, this._crosshair.y - MARGIN);
    }

    this._overlayCtx.restore();
  }

  _plot() {
    const width = this._ctx.canvas.width;
    const height = this._ctx.canvas.height;
    this._ctx.clearRect(0, 0, width, height);
    this._ctx.fillStyle = "#fff";

    // Draw the detail.
    this._ctx.save();
    this._ctx.beginPath();
    this._ctx.rect(this._detailRect.x, this._detailRect.y, this._detailRect.width, this._detailRect.height);
    this._ctx.clip();
    this._lineData.forEach(line => {
      this._ctx.strokeStyle = line._color;

      this._ctx.stroke(line.detail._linePath);
      this._ctx.fill(line.detail._dotsPath);
      this._ctx.stroke(line.detail._dotsPath);
    })
    this._ctx.restore();

    // Draw the summary.
    this._ctx.save();
    this._ctx.beginPath();
    this._ctx.rect(this._summaryRect.x, this._summaryRect.y, this._summaryRect.width, this._summaryRect.height);
    this._ctx.clip();

    this._lineData.forEach(line => {
      this._ctx.strokeStyle = line._color;

      this._ctx.stroke(line.summary._linePath);
      this._ctx.fill(line.summary._dotsPath);
      this._ctx.stroke(line.summary._dotsPath);
    })
    this._ctx.restore();

    // Draw Axes.
    const yDetailAxis = new Path2D();
    this._ctx.strokeStyle = "#000";
    this._ctx.fillStyle = "#000";
    this._yDetailRange.ticks().forEach(t => {
      const ty = this._yDetailRange(t);
      yDetailAxis.moveTo(3 * MARGIN / 4, ty);
      yDetailAxis.lineTo(MARGIN, ty);
      this._ctx.fillText('' + t, 0, ty, MARGIN / 2);
    });
    yDetailAxis.moveTo(this._detailRect.x, this._detailRect.y);
    yDetailAxis.lineTo(this._detailRect.x, this._detailRect.y + this._detailRect.height);
    this._ctx.stroke(yDetailAxis);

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

