// Setup for mocha tests in jsdom.
/* eslint-disable @typescript-eslint/no-var-requires */
/* eslint-disable @typescript-eslint/no-require-imports */
require('jsdom-global')(undefined, {
  url: 'http://localhost/',
});

const nodeFetch = require('node-fetch');
const fetchMock = require('fetch-mock');
const rfdc = require('rfdc')();

// Polyfills and globals for browser-like environment in Node
(global as any).JSCompiler_renameProperty = (p: string) => p;
(global as any).ResizeObserver = class ResizeObserver {
  observe() {}

  unobserve() {}

  disconnect() {}
};

// Mock localStorage
const localStorageMock = (() => {
  let store: { [key: string]: string } = {};
  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value.toString();
    },
    clear: () => {
      store = {};
    },
    removeItem: (key: string) => {
      delete store[key];
    },
  };
})();
Object.defineProperty(window, 'localStorage', { value: localStorageMock });
Object.defineProperty(global, 'localStorage', { value: localStorageMock });

// Explicitly attach all JSDOM window properties to global
Object.keys(window).forEach((key) => {
  if (!(key in global)) {
    (global as any)[key] = (window as any)[key];
  }
});

// Some properties are not enumerable on window but needed
const extraGlobals = [
  'Node',
  'Element',
  'HTMLElement',
  'CustomEvent',
  'MutationObserver',
  'customElements',
  'InputEvent',
  'navigator',
  'document',
  'location',
  'history',
];
extraGlobals.forEach((key) => {
  if (!(key in global)) {
    (global as any)[key] = (window as any)[key];
  }
});

(global as any).self = global;

// Improved requestAnimationFrame polyfill
(global as any).requestAnimationFrame = (callback: any) =>
  setTimeout(() => callback(Date.now()), 16);
(global as any).cancelAnimationFrame = (id: any) => clearTimeout(id);
(window as any).requestAnimationFrame = (global as any).requestAnimationFrame;
(window as any).cancelAnimationFrame = (global as any).cancelAnimationFrame;

// Mock structuredClone
if (typeof global.structuredClone === 'function') {
  (window as any).structuredClone = global.structuredClone;
} else {
  // Fallback for older Node versions (though Node 18+ is expected)
  // Note: rfdc is not fully compliant (e.g. Dates, Maps, Sets), but faster for JSON-like objects.
  (global as any).structuredClone = (obj: any) => rfdc(obj);
  (window as any).structuredClone = (global as any).structuredClone;
}

(global as any).Path2D = class Path2D {
  moveTo() {}

  lineTo() {}

  arc() {}

  closePath() {}

  bezierCurveTo() {}

  quadraticCurveTo() {}

  rect() {}

  addPath() {}
};

(global as any).sinon = require('sinon');
(window as any).sinon = (global as any).sinon;

const mockLayout = {
  getChartAreaBoundingBox: () => ({ left: 0, top: 0, width: 100, height: 100 }),
  getXLocation: (v: any) => (typeof v === 'number' ? v : 0),
  getYLocation: (v: any) => (typeof v === 'number' ? v : 0),
  getHAxisValue: (v: any) => v,
  getVAxisValue: (v: any) => v,
};

class MockDataTable {
  private _rows: any[] = [];

  private _cols: any[] = [];

  addColumn(type: string, label: string) {
    this._cols.push({ type, label });
  }

  addRow(row: any[]) {
    this._rows.push(row);
  }

  addRows(rows: any[][]) {
    this._rows.push(...rows);
  }

  getNumberOfColumns() {
    return this._cols.length;
  }

  getNumberOfRows() {
    return this._rows.length;
  }

  getFilteredRows() {
    return Array.from({ length: this._rows.length }, (_, i) => i);
  }

  getColumnLabel(i: number) {
    return this._cols[i]?.label || '';
  }

  getValue(r: number, c: number) {
    return this._rows[r] ? this._rows[r][c] : null;
  }

  getColumnIndex(label: string) {
    return this._cols.findIndex((c) => c.label === label);
  }

  getFormattedValue(r: number, c: number) {
    return String(this.getValue(r, c));
  }
}

class MockDataView {
  private data: any;

  constructor(data: any) {
    this.data = data;
  }

  setColumns() {}

  hideColumns() {}

  setRows() {}

  getViewColumns() {
    const ncols = this.data?.getNumberOfColumns() || 0;
    return Array.from({ length: ncols }, (_, i) => i);
  }

  getViewColumnIndex(i: number) {
    return i;
  }

  getColumnLabel(i: number) {
    return this.data?.getColumnLabel(i) || '';
  }

  getNumberOfRows() {
    return this.data?.getNumberOfRows() || 0;
  }

  getNumberOfColumns() {
    return this.data?.getNumberOfColumns() || 0;
  }

  getTableRowIndex(i: number) {
    return i;
  }

  getTableColumnIndex(i: number) {
    return i;
  }
}

(global as any).google = {
  visualization: {
    arrayToDataTable: (arr: any) => {
      const dt = new MockDataTable();
      const firstRow = arr[0];
      firstRow.forEach((col: any) => {
        if (typeof col === 'object') {
          dt.addColumn(col.type, col.label);
        } else {
          dt.addColumn('number', col);
        }
      });
      for (let i = 1; i < arr.length; i++) {
        dt.addRow(arr[i]);
      }
      return dt;
    },
    DataTable: MockDataTable,
    DataView: MockDataView,
    ChartWrapper: class {
      private _chart: any = null;

      setChartType() {}

      setDataTable() {}

      setOptions() {}

      setContainerId() {}

      draw() {
        this._chart = {
          getChartLayoutInterface: () => mockLayout,
          dispatchEvent: (_e: any) => {},
          addEventListener: (_n: string, _c: any) => {},
        };
      }

      getChart() {
        return this._chart;
      }
    },
    LineChart: class extends EventTarget {
      draw() {}

      getChartLayoutInterface() {
        return mockLayout;
      }

      getSelection() {
        return [];
      }

      setSelection() {}
    },
    AreaChart: class extends EventTarget {
      draw() {
        this.dispatchEvent(new CustomEvent('ready'));
      }

      getChartLayoutInterface() {
        return mockLayout;
      }
    },
    NumberFormat: class {
      format() {}
    },
    events: {
      addListener: (obj: any, eventName: string, callback: any) => {
        if (obj && obj.addEventListener) {
          obj.addEventListener(eventName, callback);
        }
      },
      trigger: (obj: any, eventName: string, detail: any) => {
        if (obj && obj.dispatchEvent) {
          obj.dispatchEvent(new CustomEvent(eventName, { detail }));
        }
      },
    },
  },
  charts: {
    load: () => Promise.resolve(),
    setOnLoadCallback: (callback: any) => callback(),
  },
};

// Define google-chart custom element
if (typeof customElements !== 'undefined') {
  class MockGoogleChart extends HTMLElement {
    private _type: string = '';

    private _data: any = null;

    private _options: any = {};

    private _view: any = null;

    public chartWrapper: any;

    constructor() {
      super();
      this.chartWrapper = new (global as any).google.visualization.ChartWrapper();
    }

    set type(v: string) {
      this._type = v;
    }

    set data(v: any) {
      this._data = v;
      this.redraw();
    }

    set options(v: any) {
      this._options = v;
    }

    set view(v: any) {
      this._view = v;
      this.redraw();
    }

    redraw() {
      setTimeout(() => {
        this.chartWrapper.draw();
        this.dispatchEvent(
          new CustomEvent('google-chart-ready', {
            bubbles: true,
            composed: true,
            detail: { chart: this.chartWrapper.getChart() },
          })
        );
      }, 0);
    }

    connectedCallback() {
      this.redraw();
    }
  }
  if (!customElements.get('google-chart')) {
    customElements.define('google-chart', MockGoogleChart);
  }
}
(window as any).google = (global as any).google;

// Mock window methods
window.open = () => null;
window.confirm = () => true;
window.alert = () => {};

// Mock HTMLDialogElement
if (typeof HTMLDialogElement !== 'undefined') {
  HTMLDialogElement.prototype.show = function () {
    this.setAttribute('open', '');
  };
  HTMLDialogElement.prototype.showModal = function () {
    this.setAttribute('open', '');
  };
  HTMLDialogElement.prototype.close = function () {
    this.removeAttribute('open');
  };
}

// Mock ElementInternals
if (typeof HTMLElement !== 'undefined') {
  (HTMLElement.prototype as any).attachInternals = function () {
    return {
      setFormValue: () => {},
      setValidity: () => {},
      checkValidity: () => true,
      reportValidity: () => true,
      form: null,
      labels: [],
      validationMessage: '',
      validity: {
        badInput: false,
        customError: false,
        patternMismatch: false,
        rangeOverflow: false,
        rangeUnderflow: false,
        stepMismatch: false,
        tooLong: false,
        tooShort: false,
        typeMismatch: false,
        valid: true,
        valueMissing: false,
      },
      willValidate: false,
      ariaAtomic: null,
      ariaAutoComplete: null,
      ariaBusy: null,
      ariaChecked: null,
      ariaColCount: null,
      ariaColIndex: null,
      ariaColSpan: null,
      ariaCurrent: null,
      ariaDisabled: null,
      ariaExpanded: null,
      ariaHasPopup: null,
      ariaHidden: null,
      ariaKeyShortcuts: null,
      ariaLabel: null,
      ariaLevel: null,
      ariaLive: null,
      ariaModal: null,
      ariaMultiLine: null,
      ariaMultiSelectable: null,
      ariaOrientation: null,
      ariaPlaceholder: null,
      ariaPosInSet: null,
      ariaPressed: null,
      ariaReadOnly: null,
      ariaRelevant: null,
      ariaRequired: null,
      ariaRoleDescription: null,
      ariaRowCount: null,
      ariaRowIndex: null,
      ariaRowSpan: null,
      ariaSelected: null,
      ariaSetSize: null,
      ariaSort: null,
      ariaValueMax: null,
      ariaValueMin: null,
      ariaValueNow: null,
      ariaValueText: null,
      role: null,
    };
  };
}

// Mock HTMLCanvasElement properties
if (typeof HTMLCanvasElement !== 'undefined') {
  Object.defineProperty(HTMLCanvasElement.prototype, 'width', {
    get() {
      return this._width || 300;
    },
    set(v) {
      this._width = v;
    },
    configurable: true,
  });
  Object.defineProperty(HTMLCanvasElement.prototype, 'height', {
    get() {
      return this._height || 150;
    },
    set(v) {
      this._height = v;
    },
    configurable: true,
  });
  (HTMLCanvasElement.prototype as any).getContext = function () {
    return {
      canvas: this,
      fillRect: () => {},
      clearRect: () => {},
      getImageData: (_x: any, _y: any, w: any, h: any) => ({
        data: new Uint8ClampedArray(w * h * 4),
      }),
      putImageData: () => {},
      createImageData: () => ({ data: [] }),
      setTransform: () => {},
      drawImage: () => {},
      save: () => {},
      restore: () => {},
      beginPath: () => {},
      moveTo: () => {},
      lineTo: () => {},
      closePath: () => {},
      stroke: () => {},
      fill: () => {},
      arc: () => {},
      fillText: () => {},
      measureText: () => ({ width: 0 }),
      transform: () => {},
      rect: () => {},
      clip: () => {},
      setLineDash: () => {},
    };
  };
}

// Mock visualViewport
(window as any).visualViewport = {
  addEventListener: () => {},
  removeEventListener: () => {},
  width: 1024,
  height: 768,
  offsetLeft: 0,
  offsetTop: 0,
  pageLeft: 0,
  pageTop: 0,
  scale: 1,
};
(global as any).visualViewport = (window as any).visualViewport;

// Mock for Request/Response/Headers
(global as any).Request = nodeFetch.Request;
(global as any).Response = nodeFetch.Response;
(global as any).Headers = nodeFetch.Headers;
(window as any).Request = nodeFetch.Request;
(window as any).Response = nodeFetch.Response;
(window as any).Headers = nodeFetch.Headers;

// Patch global fetch to handle relative URLs
const fetchHandler = (url: any, config: any) => {
  let urlStr = typeof url === 'string' ? url : url.url;
  if (urlStr.startsWith('/')) {
    urlStr = `http://localhost${urlStr}`;
  }
  // Use the singleton fetchMock to handle the request
  // We use fetchHandler property which is the callable part in v9
  return fetchMock.fetchHandler(urlStr, config);
};
if (!window.fetch || !(window.fetch as any).isSinonProxy) {
  (window as any).fetch = fetchHandler;
  (global as any).fetch = fetchHandler;
}

fetchMock.config.fallbackToNetwork = false;
fetchMock.config.overwriteRoutes = false;
fetchMock.config.warnOnFallback = false;

const registerGlobalMocks = () => {
  // Use a fallback that returns a resolved promise with an error body
  // instead of a rejected promise, to prevent "Uncaught error" in some environments.
  fetchMock.catch((url: any, options: any) => {
    const urlStr = url.toString();
    const method = (options && options.method) || 'GET';

    if (urlStr.includes('/_/fe_telemetry')) return { status: 200, body: {} };
    if (urlStr.includes('/_/shortcut/update'))
      return { status: 200, body: { id: 'new-shortcut-id' } };
    if (urlStr.includes('/_/nextParamList/'))
      return { status: 200, body: { count: 0, paramset: {} } };
    if (urlStr.includes('/_/initpage/'))
      return { status: 200, body: { dataframe: { paramset: {}, header: [] } } };
    if (urlStr.includes('/_/login/status'))
      return { status: 200, body: { email: 'user@google.com', roles: ['editor'] } };
    if (urlStr.includes('/_/defaults/')) {
      return {
        status: 200,
        body: {
          radius: 10,
          k: 50,
          algo: 'kmeans',
          interesting: 0,
          step_up_only: false,
          direction: 'BOTH',
          min_num_points: 0,
          conditional_defaults: [],
        },
      };
    }
    if (urlStr.includes('/_/anomalies/group_report'))
      return { status: 200, body: { timerange_map: {} } };
    if (urlStr.includes('/_/cidRange/'))
      return { status: 200, body: { commitSlice: [{ hash: 'abc' }] } };
    if (urlStr.includes('/_/triage/')) return { status: 200, body: {} };
    if (urlStr.includes('/_/links/')) return { status: 200, body: {} };
    if (urlStr.includes('/_/details/')) return { status: 200, body: {} };

    return { status: 404, body: { error: `Unmocked fetch ${method} call to ${urlStr}` } };
  });
};

registerGlobalMocks();

const originalRestore = fetchMock.restore.bind(fetchMock);
fetchMock.restore = () => {
  const res = originalRestore();
  registerGlobalMocks();
  return res;
};

// Mock for window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => {},
  }),
});

window.scrollTo = () => {};

// Mock window.perf
(window as any).perf = {
  commit_range_url: 'https://github.com/google/skia/compare/{begin}...{end}',
  key_order: ['benchmark', 'bot', 'test', 'subtest_1', 'subtest_2', 'subtest_3'],
  demo: true,
  need_alert_action: false,
};

(global as any).getComputedStyle = window.getComputedStyle;

// Mock for problematic ESM modules
const googleChartMock = {
  GoogleChart: (customElements as any).get('google-chart'),
  load: () => Promise.resolve(),
};
const Module = require('module');
const originalLoad = Module._load;
Module._load = function (...args: any[]) {
  if (args[0] === '@google-web-components/google-chart') {
    return googleChartMock;
  }
  return originalLoad.apply(this, args);
};
