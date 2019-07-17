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
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { Chart } from 'chart.js'
import 'chartjs-plugin-annotation'
import 'chartjs-plugin-zoom'

/**
 * @constant {string} - Prefix for trace ids that are not real traces.
 */
const SPECIAL = 'special';

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

const template = (ele) => html`
  <canvas width=${ele.width} height=${ele.height}></canvas>
`;

window.customElements.define('plot-simple-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();

    // Only create the _chart once.
    if (!this._chart) {
      this._render();

      // The location of the XBar. See setXBar().
      this._xbarx = 0;

      // The locations of the background bands. See setBanding().
      this._bands = [];

      this._chart = new Chart(this.querySelector('canvas'), {
        type: 'line',
        data: {
          datasets: [],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          spanGaps: true,
          animation: {
            duration: 0, // general animation time
          },
          hover: {
            animationDuration: 0, // duration of animations when hovering an item
          },
          annotation: {
            annotations: [],
          },
          responsiveAnimationDuration: 0, // animation duration after a resize
          elements: {
            line: {
              tension: 0 // disables bezier curves
            }
          },
          tooltips: {
            intersect: false,
            mode: 'nearest',
            animationDuration: 0,
            caretPadding: 10,
            callbacks: {
              label: (tooltipItem, data) => {
                var label = data.datasets[tooltipItem.datasetIndex].label || '';
                let detail = {
                  id: label,
                  value: tooltipItem.value,
                  index: tooltipItem.index,
                  pt: [tooltipItem.index, tooltipItem.value],
                };
                this.dispatchEvent(new CustomEvent('trace_focused', {detail: detail, bubbles: true}));

                return `Value: ${tooltipItem.value}`;
              }
            },
          },
          scales: {
            xAxes: [{
              type: 'time',
              position: 'bottom',
              time: {
                source: 'labels',
                displayFormats: {
                  'millisecond': 'h:mm:ss.SSS A',
                  'second': 'h:mm:ss A',
                  'minute': 'h:mm A',
                  'hour': 'ddd h A',
                  'day': 'ddd h A',
                  'week': 'D MMM',
                  'month': 'D MMM',
                },
              },
              distribution: 'series',
              ticks: {
                autoSkip: true,
                autoSkipPadding: 10,
                source: 'data',
                minRotation: 60,
                autoSkip: true,
                maxTicksLimit: 10,
              },
            }]
          },
          legend: {
            display: false,
          },
          onClick: (e) => {
            let eles = this._chart.getElementAtEvent(e);
            if (!eles.length) {
              return
            }
            let ele = eles[0];
            let id = this._chart.data.datasets[ele._datasetIndex].label;
            if (id.startsWith(SPECIAL))  {
              return
            }
            let index = ele._index;
            let value = this._chart.data.datasets[ele._datasetIndex].data[ele._index];
            let detail =  {
              id: id,
              index: index,
              value: value,
              pt: [index, value],
            };
            this.dispatchEvent(new CustomEvent('trace_selected', {detail: detail, bubbles: true}));
            this.setHighlight([id]);
          },
          plugins: {
            zoom: {
              pan: {
                enabled: false,
              },
              zoom: {
                enabled: true,
                drag: true,

                drag: {
                  borderColor: 'lightgray',
                  borderWidth: 3,
                },

                mode: 'xy',
                rangeMin: {
                  x: null,
                  y: null
                },
                rangeMax: {
                  x: null,
                  y: null
                },

                // Speed of zoom via mouse wheel
                // (percentage of zoom on a wheel event)
                speed: 0.1,

                onZoom: (c) => {
                  console.log(c.chart.scales);
                  let detail = {
                    xMin: c.chart.scales['x-axis-0'].min,
                    xMax: c.chart.scales['x-axis-0'].max,
                    yMin: c.chart.scales['y-axis-0'].min,
                    yMax: c.chart.scales['y-axis-0'].max,
                  };
                  this.dispatchEvent(new CustomEvent('zoom', {detail: detail, bubbles: true}));
                },
              }
            }
          }
        },
      });
    }
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
    var hash = 0;
    for (var i = s.length - 1; i >= 0; i--) {
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
   *     var lines = {
   *       foo: [
   *         [0.1, 3.7],
   *         [0.2, 3.8],
   *         [0.4, 3.0],
   *       ],
   *       bar: [
   *         [0.0, 2.5],
   *         [0.2, 4.2],
   *         [0.5, 3.9],
   *       ],
   *     };
   *     var labels = [new Date(), new Date()];
   *     plot.addLines(lines, labels);
   */
  addLines(lines, labels) {
    if (labels) {
      this._chart.data.labels = labels;
      let unit = this._diffDateToUnits(labels[0], labels[labels.length-1]);
      this._chart.options.scales.xAxes[0].time.unit = unit;
    }

    Object.keys(lines).forEach((id) => {
      let data = lines[id].map(arr => arr[1]);
      this._chart.data.datasets.push({
        label: id,
        data: data,
        fill: false,
        borderColor: colors[(this._hashString(id) % 8) + 1],
        borderWidth: 1,
      });
    });
    this._chart.update();
  }

  /**
   * Delete a line from being plotted.
   *
   * @param {string} id - The trace id.
   */
  deleteLine(id) {
    let ds = this._chart.data.datasets;
    for (var i = 0; i < ds.length; i++) {
      if (ds[i].label === id) {
        this._chart.data.datasets.splice(i, 1);
        this._chart.update();
        return;
      }
    }
  }

  /**
   * Remove all lines from plot.
   */
  removeAll() {
    this._chart.data.datasets = [];
    this._chart.update();
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
   *     var bands = [
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
      this._render();
    }
  }

});

