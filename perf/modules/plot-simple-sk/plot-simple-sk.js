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
import { Chart } from 'chart.js'
import 'chartjs-plugin-annotation'
import 'chartjs-plugin-zoom'
import * as d3 from 'd3'

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

const template = (ele) => html`
  <canvas class=traces width=${ele.width} height=${ele.height}></canvas>
`;

define('plot-simple-sk', class extends ElementSk {
  constructor() {
    super(template);

    // The location of the XBar. See setXBar().
    this._xbarx = 0;

    // The locations of the background bands. See setBanding().
    this._bands = [];

    this._lineData = [];

    this._ctx = null;
    this._clientRect = null;

    this._mouseMoveRaw = null;

    // Do we use two canvas's for the summary
    // and the detail?

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
      };
    });
    window.requestAnimationFrame(this._raf.bind(this));
  }

  _raf() {
    if (this._mouseMoveRaw != null) {
      const x = this._mouseMoveRaw.x - this._clientRect.left;
      const y = this._mouseMoveRaw.y - this._clientRect.top;
      const sx = this._xDetailRange.invert(x);
      const sy = this._yDetailRange.invert(y);
      console.log(sx, sy);
      this._mouseMoveRaw = null;
    }
    window.requestAnimationFrame(this._raf.bind(this));
  }

  // Convert the different in time between d1 and d2 into the units to
  // when displaying ticks. See https://www.chartjs.org/docs/latest/axes/cartesian/time.html#display-formats
  // and https://momentjs.com/docs/#/displaying/format/
  //
  // This works in coordination with the values set in time.displayFormats.
  _diffDateToUnits(d1, d2) {
    let diff = d2 - d1;
    diff = diff / 1000;
    if (diff < 1) {
      return 'millisecond';
    }
    diff = diff / 60;
    if (diff < 1) {
      return 'second';
    }
    diff = diff / 60;
    if (diff < 1) {
      return 'minute';
    }
    diff = diff / 24;
    if (diff < 1) {
      return 'hour';
    }
    diff = diff / 7;
    if (diff < 1) {
      return 'day';
    }
    diff = diff / 365;
    if (diff < 1) {
      return 'week';
    }
    return 'month';
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

    this._updateScaleDomains();
    this._recalcPaths();
    this._plot();
  }

  _recalcPaths() {
    this._lineData.forEach(line => {
      // Need to pass in the x and y ranges, and the dot radius.
      line._color = colors[(this._hashString(line.name) % 8) + 1];

      const detailBuilder = new PathBuilder(this._xDetailRange, this._yDetailRange, RADIUS);
      const summaryBuilder = new PathBuilder(this._xSummaryRange, this._ySummaryRange, SUMMARY_RADIUS);

      line.values.forEach((y, x) => {
        if (isNaN(y)) {
          return
        }
        detailBuilder.add(x, y)
        summaryBuilder.add(x, y)
      });
      line.detail = detailBuilder.paths();
      line.summary = summaryBuilder.paths();
    })
  }

  _updateScaleDomains() {
    this._xDetailRange = this._xDetailRange
      .domain([0, this._lineData[0].values.length]);

    this._xSummaryRange = this._xSummaryRange
      .domain([0, this._lineData[0].values.length]);

    const domain = [
      d3.min(this._lineData, line => d3.min(line.values)),
      d3.max(this._lineData, line => d3.max(line.values))
    ];

    this._yDetailRange = this._yDetailRange
      .domain(domain);

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
      y: SUMMARY_HEIGHT + 2*MARGIN,
      width: width - 2*MARGIN,
      height: height - SUMMARY_HEIGHT - 3*MARGIN,
    }

    // Also update the mousemove watcher with the rect
    // of the Detail.
    // Also create a Path2D rect of the Detail for use
    // as a clip.
  }

  _plot() {
    const width = this._ctx.canvas.width;
    const height = this._ctx.canvas.height;
    this._ctx.clearRect(0, 0, width, height);
    this._lineData.forEach(line => {
      this._ctx.strokeStyle = line._color;
      this._ctx.fillStyle = "#fff";

      this._ctx.save();
      this._ctx.beginPath();
      this._ctx.rect(this._detailRect.x, this._detailRect.y, this._detailRect.width, this._detailRect.height);
      this._ctx.clip();
      this._ctx.stroke(line.detail._linePath);
      this._ctx.fill(line.detail._dotsPath);
      this._ctx.stroke(line.detail._dotsPath);
      this._ctx.restore();

      this._ctx.save();
      this._ctx.beginPath();
      this._ctx.rect(this._summaryRect.x, this._summaryRect.y, this._summaryRect.width, this._summaryRect.height);
      this._ctx.clip();
      this._ctx.stroke(line.summary._linePath);
      this._ctx.fill(line.summary._dotsPath);
      this._ctx.stroke(line.summary._dotsPath);
      this._ctx.restore();
    })
  }

  /**
   * Delete a line from being plotted.
   *
   * @param {string} id - The trace id.
   */
  deleteLine(id) {
    let ds = this._chart.data.datasets;
    for (let i = 0; i < ds.length; i++) {
      if (ds[i].label === id) {
        this._chart.data.datasets.splice(i, 1);
      }
    }
    this._chart.update();
  }

  /**
   * Remove all lines from plot.
   */
  removeAll() {
    this._lineData = [];
    this._plot();
  }

  /**
   * Highlight one or more traces.
   *
   * @param {Array} ids - An array of trace ids.
   */
  setHighlight(ids) {
    this._chart.data.datasets.forEach((dataset) => {
      if (ids.indexOf(dataset.label) != -1) {
        dataset.borderWidth = 3;
      } else {
        dataset.borderWidth = 1;
      }
    });
    this._chart.update();
  }

  /**
   * Returns the trace ids of all highlighted traces.
   *
   * @return {Array} Trace ids.
   */
  highlighted() {
    let h = [];
    this._chart.data.datasets.forEach((dataset) => {
      if (dataset.borderWidth === 3) {
        h.push(dataset.label);
      }
    });
    return h;
  }

  /**
   * Clears all highlighting from all traces.
   */
  clearHighlight() {
    this._chart.data.datasets.forEach((dataset) => {
      dataset.borderWidth = 1;
    });
    this._chart.update();
  }

  /**
   * Turns on a vertical bar at the given position.
   *
   * @param {Number} x - The offset into the labels where the bar
   *   should be positioned.
   */
  setXBar(x) {
    this.clearXBar();
    this._chart.options.annotation.annotations.push({
      id: 'xbar',
      type: 'line',
      mode: 'vertical',
      scaleID: 'x-axis-0',
      value: this._chart.data.labels[x],
      borderColor: 'red',
      borderWidth: 3,
      drawTime: 'beforeDatasetsDraw',
    });
    this._chart.update();
  }

  /**
   * Removes the x-bar from being displayed.
   *
   * @param {Number} x - The offset into the labels where the bar
   *   should be removed from.
   */
  clearXBar(x) {
    this._chart.options.annotation.annotations =
      this._chart.options.annotation.annotations.filter(ann => {
        return ann.id != 'xbar';
      });
    this._chart.update();
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
    this._chart.options.annotation.annotations = [];
    bands.forEach((band, i) => {
      this._chart.options.annotation.annotations.push({
        id: `band-${i}`,
        type: 'box',
        mode: 'vertical',
        xScaleID: 'x-axis-0',
        xMin: this._chart.data.labels[band[0]],
        xMax: this._chart.data.labels[band[1]],
        backgroundColor: 'rgba(0, 0, 0, 0.1)',
        drawTime: 'beforeDatasetsDraw',
      });
    });
    this._chart.update();
  }

  /**
   * Resets the zoom to default.
   */
  resetAxes() {
    if (this._chart) {
      this._chart.resetZoom();
    }
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
    if (canvas) {
      this._clientRect = canvas.getBoundingClientRect();
      this._ctx = canvas.getContext('2d');
      this._updateScaleRanges();
      this._recalcPaths();
      this._plot();
    }
  }

});

