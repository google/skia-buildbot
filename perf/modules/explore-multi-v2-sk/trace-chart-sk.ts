import { LitElement, css, html } from 'lit';
import { customElement, property, query, state } from 'lit/decorators.js';
import {
  normalizeValues,
  calculateMeasurement,
  computeDiffParamNames,
  computeChartDimensions,
  computeLeftPadding,
} from './chart-logic';
import { smoothPoints } from './smoothing';
import { Regression } from '../json';
import { TrimHash } from '../common/commit';
import './trace-chart-tooltip-sk';

export interface TraceRow {
  commit_number: number;
  val: number;
  createdat: number;
  metadata?: Record<string, string> | null;
  hash?: string;
  url?: string;
  smoothedVal?: number;
}

export interface TraceSeries {
  id: string;
  source?: string;
  rows: TraceRow[];
  allStats?: Record<string, TraceRow[]>;
  color: string;
}

export interface ProcessedTraceSeries extends TraceSeries {
  parsedColor: { r: number; g: number; b: number };
}

// Returns a fractional index representing the value's relative position between two actual indices.
// For example, if val is halfway between arr[i] and arr[i+1], it returns i + 0.5.
function getVirtualIndex(arr: number[], val: number): number {
  if (arr.length === 0) return 0;
  if (val <= arr[0]) return 0;
  if (val >= arr[arr.length - 1]) return arr.length - 1;

  let low = 0;
  let high = arr.length - 1;
  while (low <= high) {
    const mid = Math.floor((low + high) / 2);
    if (arr[mid] === val) return mid;
    if (arr[mid] < val) low = mid + 1;
    else high = mid - 1;
  }
  // low is guaranteed to be at least 1 here as we already handled the case where val <= arr[0].
  // Thus, i will be at least 0.
  const i = low - 1;
  const frac = (val - arr[i]) / (arr[i + 1] - arr[i]);
  return i + frac;
}

function getXFromVirtualIndex(arr: number[], vIdx: number): number {
  if (arr.length === 0) return 0;
  if (vIdx <= 0) return arr[0];
  if (vIdx >= arr.length - 1) return arr[arr.length - 1];

  const i = Math.floor(vIdx);
  const frac = vIdx - i;
  return arr[i] + frac * (arr[i + 1] - arr[i]);
}

// Styling and layout constants for subrepo update rollout drawing
const ROLLOUT_COLOR = 'rgba(99, 102, 241, 0.8)'; // Indigo indicator line and label box background
const ROLLOUT_LINE_WIDTH = 1.5;
const ROLLOUT_LABEL_FONT = '10px sans-serif';
const ROLLOUT_TEXT_COLOR = '#fff'; // White text color for readability against the dark box
const ROLLOUT_LABEL_TOP_OFFSET = 5; // Pixels down from padding.top for the label stack start
const ROLLOUT_LABEL_STAGGER_OFFSET = 15; // Vertical distance between staggered labels to prevent overlaps
const ROLLOUT_BOX_PADDING_X = 4; // Horizontal padding around text inside the label box
const ROLLOUT_BOX_PADDING_Y = 2; // Vertical padding around text inside the label box
const ROLLOUT_BOX_HEIGHT = 14; // Total height of the label background box

@customElement('trace-chart-sk')
export class TraceChartSk extends LitElement {
  @property({ type: String }) title = '';

  @property({ type: Number }) canvasHeight = 250;

  @property({ type: String }) yAxisLabel = 'score';

  @property({ type: Boolean }) evenXAxisSpacing = false;

  @property({ type: Array, attribute: false }) series: TraceSeries[] = [];

  @property({ type: String }) normalizeCentre: 'none' | 'first' | 'average' | 'median' = 'none';

  @property({ type: Object }) activeStats: Set<string> = new Set(['min', 'max']);

  @property({ type: Boolean }) dateMode = false;

  @property({ type: String }) normalizeScale:
    | 'none'
    | 'minmax'
    | 'stddev'
    | 'iqr'
    | 'smoothed_std' = 'none';

  private get _xAccessor() {
    return (r: TraceRow) => (this.dateMode ? r.createdat : r.commit_number);
  }

  @property() hoverMode: 'original' | 'smoothed' | 'both' = 'original';

  @property({ type: Number }) smoothingRadius = 20;

  @property({ type: Number }) edgeDetectionFactor = 1.0;

  @property({ type: Number }) edgeLookahead = 3;

  @property({ type: Boolean }) showDots = true;

  @property({ type: Boolean }) tooltipDiffs = false;

  @property({ type: Number }) viewportMinX: number | null = null;

  @property({ type: Number }) viewportMaxX: number | null = null;

  @property({ type: Number }) globalHoverX: number | null = null;

  @property({ type: Number }) globalPinnedX: number | null = null;

  @state() private _diffNamesMap: Map<string, string> = new Map();

  private _canvasWidth: number = 0;

  private _resizeObserver: ResizeObserver | null = null;

  private _xValueToIndex: Map<number, number> = new Map();

  private _sortedXValues: number[] = [];

  @property({ type: String }) selectedSubrepo: string = 'none';

  @property({ type: String }) user_id = '';

  @state() show_bisect_button = !!(window as any).perf?.show_bisect_btn;

  @state() private _show_pinpoint_buttons = !!(window as any).perf?.show_bisect_btn;

  @state() private _isMouseOverTooltip = false;

  @property({ type: Array, attribute: false }) activeSplitKeys: string[] = [];

  @state() private _potentialSplitKeys: string[] = [];

  @state() private _subrepoRolls: { dataX: number; oldVer: string; newVer: string }[] = [];

  @property({ type: Object, attribute: false }) regressions: {
    [trace_id: string]: { [commit: number]: Regression };
  } = {};

  @property({ type: Object, attribute: false }) loadedBounds: Record<
    string,
    { min: number; max: number }
  > = {};

  @property({ type: Object, attribute: false }) globalBounds: Record<
    string,
    { min: number; max: number }
  > = {};

  @property({ type: Boolean }) isSparkline = false;

  @property({ type: Boolean }) loading = false;

  @query('#chart-canvas')
  private canvas!: HTMLCanvasElement;

  @query('#overlay-canvas')
  private overlayCanvas!: HTMLCanvasElement;

  @state()
  private _hoveredPoint: { series: TraceSeries; row: TraceRow; x: number; y: number } | null = null;

  @state()
  private _mousePos: { x: number; y: number } | null = null;

  @state()
  private _measureState: {
    startX: number;
    startY: number;
    currentX: number;
    currentY: number;
    startDataY: number;
    currentDataY: number;
  } | null = null;

  @state() private _viewportMinX: number | null = null;

  @state() private _viewportMaxX: number | null = null;

  @state() private _selectedRange: {
    minCommit: number;
    maxCommit: number;
    startX: number;
    endX: number;
  } | null = null;

  private _dragCtx = {
    isDragging: false,
    dragStartX: 0,
    dragStartY: 0,
    dragStartMinY: 0,
    dragStartMaxY: 0,
    isShift: false,
    isCtrl: false,
    currentX: 0,
    pointerId: -1,
  };

  private _processedSeries: ProcessedTraceSeries[] = [];

  private _computeSubrepoRolls(): { dataX: number; oldVer: string; newVer: string }[] {
    if (!this.selectedSubrepo || this.selectedSubrepo === 'none') return [];
    const rolls: { dataX: number; oldVer: string; newVer: string }[] = [];
    const seen = new Set<number>();

    this.series.forEach((s) => {
      let prevVer: string | null = null;
      s.rows.forEach((r) => {
        const currVer = r.metadata?.[this.selectedSubrepo];
        if (currVer && prevVer && currVer !== prevVer) {
          if (!seen.has(r.commit_number)) {
            seen.add(r.commit_number);
            rolls.push({ dataX: this._xAccessor(r), oldVer: prevVer, newVer: currVer });
          }
        }
        if (currVer) {
          prevVer = currVer;
        }
      });
    });
    return rolls;
  }

  connectedCallback() {
    super.connectedCallback();
    this._resizeObserver = new ResizeObserver(() => {
      this._drawBackground();
      this._drawForeground();
    });
    this._resizeObserver.observe(this);
  }

  disconnectedCallback() {
    if (this._resizeObserver) {
      this._resizeObserver.disconnect();
    }
    super.disconnectedCallback();
  }

  updated(changedProperties: Map<string | number | symbol, unknown>) {
    if (changedProperties.has('series')) {
      this._diffNamesMap = computeDiffParamNames(this.series);
      this._potentialSplitKeys = computeChartDimensions(this.series);
    }

    if (changedProperties.has('series') || changedProperties.has('selectedSubrepo')) {
      this._subrepoRolls = this._computeSubrepoRolls();
    }

    if (changedProperties.has('series') || changedProperties.has('dateMode')) {
      const uniqueXValues = new Set<number>();
      this.series.forEach((s) => {
        s.rows.forEach((r) => {
          uniqueXValues.add(this._xAccessor(r));
        });
      });
      this._sortedXValues = Array.from(uniqueXValues).sort((a, b) => a - b);
      this._xValueToIndex = new Map<number, number>();
      this._sortedXValues.forEach((val, idx) => this._xValueToIndex.set(val, idx));
    }

    if (
      changedProperties.has('series') ||
      changedProperties.has('normalizeCentre') ||
      changedProperties.has('normalizeScale') ||
      changedProperties.has('hoverMode') ||
      changedProperties.has('smoothingRadius') ||
      changedProperties.has('edgeDetectionFactor') ||
      changedProperties.has('edgeLookahead')
    ) {
      this._processedSeries = this.series.map((s) => {
        let r_c = 26,
          g_c = 115,
          b_c = 232;
        const baseColor = s.color || '#1a73e8';
        if (baseColor.startsWith('#') && baseColor.length === 7) {
          r_c = parseInt(baseColor.slice(1, 3), 16);
          g_c = parseInt(baseColor.slice(3, 5), 16);
          b_c = parseInt(baseColor.slice(5, 7), 16);
        }
        const parsedColor = { r: r_c, g: g_c, b: b_c };

        if (s.rows.length === 0) return { ...s, rows: [], parsedColor };

        let seriesMinY = Infinity;
        let seriesMaxY = -Infinity;
        s.rows.forEach((r) => {
          if (r.val < seriesMinY) seriesMinY = r.val;
          if (r.val > seriesMaxY) seriesMaxY = r.val;
        });
        const seriesRangeY = seriesMaxY - seriesMinY || 1;

        let smoothedValues: number[] | null = null;
        if (this.hoverMode !== 'original') {
          const stablePoints = s.rows.map((r) => ({
            px: r.commit_number,
            py: ((r.val - seriesMinY) / seriesRangeY) * 1000,
            rawPy: ((r.val - seriesMinY) / seriesRangeY) * 1000,
            rawX: r.commit_number,
          }));
          const res = smoothPoints(
            stablePoints,
            this.smoothingRadius,
            this.edgeDetectionFactor,
            this.edgeLookahead
          );
          smoothedValues = res.smoothed;
        }

        let norm: { offset: number; scale: number };
        const useSmoothedForNorm = this.hoverMode === 'smoothed';
        if (useSmoothedForNorm && smoothedValues) {
          const smoothedReal = smoothedValues.map((v) => seriesMinY + (v / 1000) * seriesRangeY);
          norm = normalizeValues(
            smoothedReal,
            this.normalizeCentre,
            this.normalizeScale,
            0,
            smoothedReal.length - 1
          );
        } else {
          norm = normalizeValues(
            s.rows,
            this.normalizeCentre,
            this.normalizeScale,
            0,
            s.rows.length - 1
          );
        }

        const allStats = s.allStats ? { ...s.allStats } : {};
        const errRows = allStats['err'] || allStats['error'];
        if (errRows) {
          const yValsByCommit = new Map<number, number>();
          s.rows.forEach((r) => yValsByCommit.set(r.commit_number, r.val));

          const stdMinRows: TraceRow[] = [];
          const stdMaxRows: TraceRow[] = [];

          errRows.forEach((r) => {
            const yVal = yValsByCommit.get(r.commit_number);
            if (yVal !== undefined) {
              stdMinRows.push({ ...r, val: yVal - r.val });
              stdMaxRows.push({ ...r, val: yVal + r.val });
            }
          });

          allStats['stdMin'] = stdMinRows;
          allStats['stdMax'] = stdMaxRows;
        }

        const mappedAllStats: Record<string, TraceRow[]> = {};
        for (const [key, rows] of Object.entries(allStats)) {
          mappedAllStats[key] = rows.map((r) => ({
            ...r,
            val: (r.val - norm.offset) * (norm.scale || 1),
          }));
        }

        return {
          ...s,
          parsedColor,
          rows: s.rows.map((r, i) => {
            const rawY = r.val;
            const smoothY = smoothedValues
              ? seriesMinY + (smoothedValues[i] / 1000) * seriesRangeY
              : rawY;
            return {
              ...r,
              val: (rawY - norm.offset) * (norm.scale || 1),
              smoothedVal: (smoothY - norm.offset) * (norm.scale || 1),
            };
          }),
          allStats: Object.keys(mappedAllStats).length > 0 ? mappedAllStats : undefined,
        };
      });
    }

    if (changedProperties.has('viewportMinX') && this.viewportMinX === null) {
      this._viewportMinX = null;
    }
    if (changedProperties.has('viewportMaxX') && this.viewportMaxX === null) {
      this._viewportMaxX = null;
    }

    let needsBackgroundRedraw = false;
    let needsForegroundRedraw = false;

    if (changedProperties.has('viewportMinX') && this.viewportMinX !== null) {
      this._viewportMinX = this.viewportMinX;
    }
    if (changedProperties.has('viewportMaxX') && this.viewportMaxX !== null) {
      this._viewportMaxX = this.viewportMaxX;
    }

    if (
      changedProperties.has('series') ||
      changedProperties.has('normalizeCentre') ||
      changedProperties.has('normalizeScale') ||
      changedProperties.has('hoverMode') ||
      changedProperties.has('smoothingRadius') ||
      changedProperties.has('showDots') ||
      changedProperties.has('viewportMinX') ||
      changedProperties.has('viewportMaxX') ||
      changedProperties.has('_viewportMinX') ||
      changedProperties.has('_viewportMaxX') ||
      changedProperties.has('activeStats') ||
      changedProperties.has('selectedSubrepo') ||
      changedProperties.has('evenXAxisSpacing')
    ) {
      needsBackgroundRedraw = true;
    }

    if (
      needsBackgroundRedraw ||
      changedProperties.has('_hoveredPoint') ||
      changedProperties.has('_mousePos') ||
      changedProperties.has('_measureState') ||
      changedProperties.has('globalHoverX') ||
      changedProperties.has('globalPinnedX')
    ) {
      needsForegroundRedraw = true;
    }

    if (needsBackgroundRedraw) this._drawBackground();
    if (needsForegroundRedraw) this._drawForeground();
  }

  private _setupCanvas(canvas: HTMLCanvasElement, rect: DOMRect): CanvasRenderingContext2D | null {
    const ctx = canvas.getContext('2d');
    if (!ctx) return null;
    const dpr = window.devicePixelRatio || 1;
    const width = rect.width;
    const height = rect.height || this.canvasHeight;
    if (width === 0 || height === 0) return null;

    const targetW = Math.floor(width * dpr);
    const targetH = Math.floor(height * dpr);

    if (canvas.width !== targetW || canvas.height !== targetH) {
      canvas.width = targetW;
      canvas.height = targetH;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    } else {
      ctx.clearRect(0, 0, width, height);
    }
    return ctx;
  }

  private _formatYValue(val: number): string {
    if (val === undefined || val === null || isNaN(val)) {
      return 'N/A';
    }
    const label = this.yAxisLabel.toLowerCase();

    // Bytes
    if (label.includes('bytes') || label.includes('sizeinbytes')) {
      const absVal = Math.abs(val);
      if (absVal >= 1024 * 1024 * 1024) return (val / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
      if (absVal >= 1024 * 1024) return (val / (1024 * 1024)).toFixed(2) + ' MB';
      if (absVal >= 1024) return (val / 1024).toFixed(2) + ' KB';
      return val.toFixed(0) + ' B';
    }

    // Bytes per second
    if (label.includes('bytespersecond')) {
      const absVal = Math.abs(val);
      if (absVal >= 1024 * 1024 * 1024) return (val / (1024 * 1024 * 1024)).toFixed(2) + ' GB/s';
      if (absVal >= 1024 * 1024) return (val / (1024 * 1024)).toFixed(2) + ' MB/s';
      if (absVal >= 1024) return (val / 1024).toFixed(2) + ' KB/s';
      return val.toFixed(0) + ' B/s';
    }

    // Percentage
    if (label.includes('n%') || label.includes('%')) {
      return val.toFixed(2) + '%';
    }

    // Time (ms)
    if (label.includes('ms')) {
      const absVal = Math.abs(val);
      if (absVal >= 1000) return (val / 1000).toFixed(2) + ' s';
      return val.toFixed(2) + ' ms';
    }

    // Time (ns)
    if (label.includes('ns')) {
      const absVal = Math.abs(val);
      if (absVal >= 1e6) return (val / 1e6).toFixed(2) + ' ms';
      if (absVal >= 1e3) return (val / 1e3).toFixed(2) + ' µs';
      return val.toFixed(0) + ' ns';
    }

    // Fallback for other units or default
    if (Math.abs(val) >= 1000000) return val.toExponential(2);
    if (Math.abs(val) < 0.01 && val !== 0) return val.toExponential(2);
    return val.toFixed(2);
  }

  private _formatTooltipValue(val: number): string {
    if (val === undefined || val === null || isNaN(val)) {
      return 'N/A';
    }
    const formatted = this._formatYValue(val);
    const rawStr = val.toFixed(4);
    const rawStrStripped = rawStr.replace(/\.?0+$/, ''); // Remove trailing zeros

    // If the formatted string doesn't contain the significant part of the raw value, show it in parentheses
    if (!formatted.includes(rawStrStripped)) {
      return `${formatted} (${rawStr})`;
    }
    return formatted;
  }

  private _drawBackground() {
    const canvas = this.canvas;
    const rect = canvas?.getBoundingClientRect();
    if (!rect) return;
    const ctx = this._setupCanvas(canvas, rect);
    if (!ctx) return;

    const width = rect.width;
    this._canvasWidth = width;
    const height = rect.height || this.canvasHeight;

    const style = window.getComputedStyle(this);
    const textColor = style.getPropertyValue('--on-surface').trim() || '#f8fafc';
    const textColorSecondary = style.getPropertyValue('--on-surface-variant').trim() || '#94a3b8';
    const borderColor = style.getPropertyValue('--outline').trim() || 'rgba(255, 255, 255, 0.1)';
    const gridColor =
      style.getPropertyValue('--outline-variant').trim() || 'rgba(255, 255, 255, 0.05)';

    if (!this._processedSeries || this._processedSeries.length === 0) {
      ctx.fillStyle = '#999';
      ctx.font = '12px sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText('[No data accessible]', width / 2, height / 2);
      return;
    }

    const {
      padding,
      graphWidth,
      graphHeight,
      minX,
      maxX,
      minY,
      maxY,
      mapX,
      mapY,
      minTimestamp,
      maxTimestamp,
      countMaxY,
      mapCountY,
    } = this._getChartBoundsAndMapping(rect);

    if (minX === Infinity) {
      ctx.fillStyle = textColorSecondary;
      ctx.font = '14px "Inter", sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(
        'No data available for this trace in the selected time range',
        width / 2,
        height / 2
      );
      return;
    }

    // Draw frame border (removed for cleaner look)

    // Draw Ticks and Labels if not sparkline
    if (!this.isSparkline) {
      ctx.fillStyle = textColorSecondary;
      ctx.font = '10px "Inter", sans-serif';
      ctx.strokeStyle = borderColor;
      ctx.lineWidth = 1;

      // Y Axis
      ctx.textAlign = 'right';
      ctx.textBaseline = 'middle';

      const maxYLabel = this._formatYValue(maxY);
      const minYLabel = this._formatYValue(minY);
      const midYLabel = this._formatYValue((minY + maxY) / 2);

      ctx.fillText(maxYLabel, padding.left - 8, padding.top);
      ctx.fillText(midYLabel, padding.left - 8, padding.top + graphHeight / 2);
      ctx.fillText(minYLabel, padding.left - 8, padding.top + graphHeight);

      ctx.beginPath();
      ctx.moveTo(padding.left - 5, padding.top);
      ctx.lineTo(padding.left, padding.top);
      ctx.moveTo(padding.left - 5, padding.top + graphHeight / 2);
      ctx.lineTo(padding.left, padding.top + graphHeight / 2);
      ctx.moveTo(padding.left - 5, padding.top + graphHeight);
      ctx.lineTo(padding.left, padding.top + graphHeight);
      ctx.stroke();

      // Y Axis Title
      if (this.yAxisLabel) {
        ctx.save();
        ctx.translate(15, padding.top + graphHeight / 2);
        ctx.rotate(-Math.PI / 2);
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillStyle = textColor;
        ctx.font = 'bold 12px "Inter", sans-serif';
        ctx.fillText(this.yAxisLabel, 0, 0);
        ctx.restore();
      }

      // Grid Lines
      ctx.beginPath();
      ctx.strokeStyle = gridColor;
      ctx.setLineDash([2, 2]);
      ctx.moveTo(padding.left, padding.top);
      ctx.lineTo(padding.left + graphWidth, padding.top);
      ctx.moveTo(padding.left, padding.top + graphHeight / 2);
      ctx.lineTo(padding.left + graphWidth, padding.top + graphHeight / 2);
      ctx.moveTo(padding.left, padding.top + graphHeight);
      ctx.lineTo(padding.left + graphWidth, padding.top + graphHeight);
      ctx.stroke();
      ctx.setLineDash([]);

      // Outer Axes
      ctx.beginPath();
      ctx.strokeStyle = borderColor;
      ctx.lineWidth = 1;

      // Y Axis line
      ctx.moveTo(padding.left, padding.top);
      ctx.lineTo(padding.left, padding.top + graphHeight);

      // X Axis line (at y=0 if within bounds, or at bottom)
      const y0 = mapY(0);
      let xAxisY = padding.top + graphHeight;
      if (y0 >= padding.top && y0 <= padding.top + graphHeight) {
        xAxisY = y0;
      } else if (maxY < 0) {
        xAxisY = padding.top;
      }

      ctx.moveTo(padding.left, xAxisY);
      ctx.lineTo(padding.left + graphWidth, xAxisY);
      ctx.stroke();

      // X Axis
      ctx.textBaseline = 'top';

      const numTicks =
        this.evenXAxisSpacing && this._sortedXValues.length > 0
          ? Math.min(6, this._sortedXValues.length)
          : 6;

      const minIdx =
        this.evenXAxisSpacing && this._sortedXValues.length > 0
          ? this._xValueToIndex.get(minX) ?? 0
          : 0;
      const maxIdx =
        this.evenXAxisSpacing && this._sortedXValues.length > 0
          ? this._xValueToIndex.get(maxX) ?? this._sortedXValues.length - 1
          : 0;

      for (let i = 0; i < numTicks; i++) {
        let val: number;
        let x: number;
        let ts: number;

        if (this.evenXAxisSpacing && this._sortedXValues.length > 0) {
          const idx = Math.round(minIdx + ((maxIdx - minIdx) * i) / (numTicks - 1));
          val = this._sortedXValues[idx];
          x = mapX(val);
          ts = minTimestamp + ((maxTimestamp - minTimestamp) * i) / (numTicks - 1); // Fallback
          for (const s of this._processedSeries) {
            const r = s.rows.find((row) => this._xAccessor(row) === val);
            if (r) {
              ts = r.createdat;
              break;
            }
          }
        } else {
          val = minX + ((maxX - minX) * i) / (numTicks - 1);
          x = mapX(val);
          ts = minTimestamp + ((maxTimestamp - minTimestamp) * i) / (numTicks - 1);
        }

        const label = this.dateMode
          ? this._formatDate(ts) || Math.round(val).toString()
          : Math.round(val).toString();

        // Align edges to prevent overlap with Y axis and bleeding off edge
        if (i === 0) {
          ctx.textAlign = 'left';
        } else if (i === numTicks - 1) {
          ctx.textAlign = 'right';
        } else {
          ctx.textAlign = 'center';
        }

        ctx.fillText(label, x, padding.top + graphHeight + 12);

        ctx.beginPath();
        ctx.moveTo(x, padding.top + graphHeight);
        ctx.lineTo(x, padding.top + graphHeight + 6);
        ctx.stroke();
      }
    }

    ctx.save();
    ctx.beginPath();
    ctx.rect(padding.left, padding.top, graphWidth, graphHeight);
    ctx.clip();

    // Draw Variance Ribbons and Lines
    const showMin = this.activeStats.has('min');
    const showMax = this.activeStats.has('max');

    if (showMin || showMax) {
      this._processedSeries.forEach((s) => {
        const minRows = s.allStats ? s.allStats['min'] : null;
        const maxRows = s.allStats ? s.allStats['max'] : null;

        // Draw ribbon if both active and available
        if (showMin && showMax && minRows && maxRows) {
          ctx.beginPath();
          ctx.fillStyle = s.color || '#1a73e8';
          ctx.globalAlpha = 0.2;

          let first = true;
          maxRows.forEach((r) => {
            const px = mapX(this._xAccessor(r));
            const py = mapY(r.val);
            if (first) {
              ctx.moveTo(px, py);
              first = false;
            } else {
              ctx.lineTo(px, py);
            }
          });

          for (let i = minRows.length - 1; i >= 0; i--) {
            const r = minRows[i];
            const px = mapX(this._xAccessor(r));
            const py = mapY(r.val);
            ctx.lineTo(px, py);
          }

          ctx.closePath();
          ctx.fill();
          ctx.globalAlpha = 1.0;
        }

        // Draw individual lines if only one is active
        ctx.lineWidth = 1;
        ctx.setLineDash([4, 4]);

        if (showMin && minRows && !showMax) {
          ctx.beginPath();
          ctx.strokeStyle = s.color || '#1a73e8';
          let first = true;
          minRows.forEach((r) => {
            const px = mapX(this._xAccessor(r));
            const py = mapY(r.val);
            if (first) {
              ctx.moveTo(px, py);
              first = false;
            } else {
              ctx.lineTo(px, py);
            }
          });
          ctx.stroke();
        }

        if (showMax && maxRows && !showMin) {
          ctx.beginPath();
          ctx.strokeStyle = s.color || '#1a73e8';
          let first = true;
          maxRows.forEach((r) => {
            const px = mapX(this._xAccessor(r));
            const py = mapY(r.val);
            if (first) {
              ctx.moveTo(px, py);
              first = false;
            } else {
              ctx.lineTo(px, py);
            }
          });
          ctx.stroke();
        }

        ctx.setLineDash([]);
      });
    }

    // Draw Std Ribbons
    if (this.activeStats.has('err') || this.activeStats.has('error')) {
      this._processedSeries.forEach((s) => {
        if (s.allStats && s.allStats['stdMin'] && s.allStats['stdMax']) {
          const minRows = s.allStats['stdMin'];
          const maxRows = s.allStats['stdMax'];
          console.log(
            'Drawing std ribbon for',
            s.id,
            'stdMin:',
            minRows.length,
            'stdMax:',
            maxRows.length
          );

          ctx.beginPath();
          ctx.fillStyle = s.color || '#1a73e8';
          ctx.globalAlpha = 0.15;

          let first = true;
          maxRows.forEach((r) => {
            const px = mapX(this._xAccessor(r));
            const py = mapY(r.val);
            if (first) {
              ctx.moveTo(px, py);
              first = false;
            } else {
              ctx.lineTo(px, py);
            }
          });

          for (let i = minRows.length - 1; i >= 0; i--) {
            const r = minRows[i];
            const px = mapX(this._xAccessor(r));
            const py = mapY(r.val);
            ctx.lineTo(px, py);
          }

          ctx.closePath();
          ctx.fill();
          ctx.globalAlpha = 1.0;
        }
      });
    }

    // Draw Count Lines
    if (this.activeStats.has('count') && countMaxY > 0) {
      ctx.lineWidth = 1.5;
      this._processedSeries.forEach((s) => {
        const statRows = s.allStats ? s.allStats['count'] : null;
        if (statRows && statRows.length > 0) {
          ctx.beginPath();
          ctx.strokeStyle = s.color || '#1a73e8';
          ctx.setLineDash([6, 4]);
          ctx.globalAlpha = 0.6;

          let firstErr = true;
          statRows.forEach((sr) => {
            const bx = this._xAccessor(sr);
            if (bx < minX || bx > maxX) return;

            const val = Number(sr.val);
            const py = mapCountY(val);
            const px = mapX(bx);

            if (firstErr) {
              ctx.moveTo(px, py);
              firstErr = false;
            } else {
              ctx.lineTo(px, py);
            }
          });
          ctx.stroke();
          ctx.setLineDash([]);
          ctx.globalAlpha = 1.0;
        }
      });
    }

    // Draw Subrepo Updates
    if (this._subrepoRolls.length > 0) {
      ctx.beginPath();
      ctx.strokeStyle = ROLLOUT_COLOR;
      ctx.lineWidth = ROLLOUT_LINE_WIDTH;

      this._subrepoRolls.forEach((roll) => {
        const px = mapX(roll.dataX);
        if (px >= padding.left && px <= width - padding.right) {
          ctx.moveTo(px, padding.top);
          ctx.lineTo(px, height - padding.bottom);
        }
      });
      ctx.stroke();

      // Draw labels for rollouts
      ctx.font = ROLLOUT_LABEL_FONT;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'top';
      this._subrepoRolls.forEach((roll, index) => {
        const px = mapX(roll.dataX);
        if (px >= padding.left && px <= width - padding.right) {
          const label = `${TrimHash(roll.oldVer)}..${TrimHash(roll.newVer)}`;
          const textWidth = ctx.measureText(label).width;
          const py =
            padding.top + ROLLOUT_LABEL_TOP_OFFSET + (index % 2) * ROLLOUT_LABEL_STAGGER_OFFSET;

          // Draw background box
          ctx.fillStyle = ROLLOUT_COLOR;
          ctx.fillRect(
            px - textWidth / 2 - ROLLOUT_BOX_PADDING_X,
            py - ROLLOUT_BOX_PADDING_Y,
            textWidth + 2 * ROLLOUT_BOX_PADDING_X,
            ROLLOUT_BOX_HEIGHT
          );

          // Draw text
          ctx.fillStyle = ROLLOUT_TEXT_COLOR;
          ctx.fillText(label, px, py);
        }
      });
    }

    // Draw Traces
    this._processedSeries.forEach((s) => {
      const showOriginal = this.hoverMode === 'original' || this.hoverMode === 'both';
      const showSmoothed = this.hoverMode === 'smoothed' || this.hoverMode === 'both';

      const baseColor = s.color || '#1a73e8';
      let traceStyle: string | CanvasGradient = baseColor;

      const loaded = this.loadedBounds[s.id];
      const global = this.globalBounds[s.id];

      if (loaded && global) {
        const needsLeftFade = loaded.min > global.min;
        const needsRightFade = loaded.max < global.max;

        if (needsLeftFade || needsRightFade) {
          const globalMinPx = mapX(global.min);
          const globalMaxPx = mapX(global.max);

          if (globalMaxPx - globalMinPx > 0) {
            const grad = ctx.createLinearGradient(globalMinPx, 0, globalMaxPx, 0);

            const solidColor = `rgba(${s.parsedColor.r}, ${s.parsedColor.g}, ${s.parsedColor.b}, 1)`;
            const transparentColor = `rgba(${s.parsedColor.r}, ${s.parsedColor.g}, ${s.parsedColor.b}, 0)`;

            const range = global.max - global.min;
            const leftStop = Math.max(0, Math.min(1, (loaded.min - global.min) / range));
            const rightStop = Math.max(0, Math.min(1, (loaded.max - global.min) / range));

            if (needsLeftFade) {
              grad.addColorStop(0, transparentColor);
              grad.addColorStop(leftStop, solidColor);
            } else {
              grad.addColorStop(0, solidColor);
            }

            if (needsRightFade) {
              grad.addColorStop(rightStop, solidColor);
              grad.addColorStop(1, transparentColor);
            } else {
              grad.addColorStop(1, solidColor);
            }

            traceStyle = grad;
          }
        }
      }

      if (showOriginal) {
        ctx.beginPath();
        ctx.strokeStyle = traceStyle;
        ctx.lineWidth = 1.5;
        if (this.hoverMode === 'both') {
          ctx.globalAlpha = 0.3;
          ctx.setLineDash([2, 2]);
        }

        console.log(`[_drawBackground] series ${s.id} has ${s.rows.length} rows.`);
        let first = true;
        s.rows.forEach((r) => {
          const px = mapX(this._xAccessor(r));
          const py = mapY(r.val);
          if (first) {
            ctx.moveTo(px, py);
            first = false;
          } else {
            ctx.lineTo(px, py);
          }
        });
        ctx.stroke();
        ctx.globalAlpha = 1.0;
        ctx.setLineDash([]);
      }

      if (showSmoothed) {
        ctx.beginPath();
        ctx.strokeStyle = traceStyle;
        ctx.lineWidth = 2.0;

        let first = true;
        s.rows.forEach((r) => {
          const px = mapX(this._xAccessor(r));
          const py = mapY(r.smoothedVal !== undefined ? r.smoothedVal : r.val);
          if (first) {
            ctx.moveTo(px, py);
            first = false;
          } else {
            ctx.lineTo(px, py);
          }
        });
        ctx.stroke();
      }

      if (this.showDots) {
        ctx.fillStyle = traceStyle;
        s.rows.forEach((r) => {
          const targetY = showSmoothed && r.smoothedVal !== undefined ? r.smoothedVal : r.val;
          ctx.beginPath();
          ctx.arc(mapX(this._xAccessor(r)), mapY(targetY), 1.5, 0, 2 * Math.PI);
          ctx.fill();
        });
      }

      // Draw Regressions
      const sr = this.regressions[s.id];
      if (sr) {
        s.rows.forEach((r) => {
          const reg = sr[r.commit_number];
          if (reg) {
            const px = mapX(this._xAccessor(r));
            const targetY = showSmoothed && r.smoothedVal !== undefined ? r.smoothedVal : r.val;
            const py = mapY(targetY);

            ctx.beginPath();
            ctx.arc(px, py, 5, 0, 2 * Math.PI);

            const regAny = reg as any;

            if (regAny.recovered) {
              ctx.globalAlpha = 0.5;
            }

            if (regAny.is_improvement) {
              ctx.fillStyle = '#4caf50'; // Green
              ctx.fill();
            } else if (regAny.bug_id !== undefined && regAny.bug_id !== 0 && regAny.bug_id > 0) {
              // Triaged regression -> Red
              ctx.fillStyle = '#e53935'; // Red
              ctx.fill();
            } else if (regAny.bug_id !== undefined && regAny.bug_id < 0) {
              // Ignored -> Gray outline
              ctx.strokeStyle = '#9e9e9e';
              ctx.lineWidth = 1.5;
              ctx.stroke();
            } else {
              // Untriaged (bug_id is 0 or undefined) -> Yellow with '?'
              ctx.fillStyle = '#f4b400'; // Google Yellow
              ctx.fill();

              // Draw '?'
              ctx.fillStyle = '#000';
              ctx.font = 'bold 8px sans-serif';
              ctx.textAlign = 'center';
              ctx.textBaseline = 'middle';
              ctx.fillText('?', px, py);
            }

            ctx.globalAlpha = 1.0; // Reset opacity
          }
        });
      }
    });

    ctx.restore();
  }

  private _drawForeground() {
    const canvas = this.overlayCanvas;
    const rect = canvas?.getBoundingClientRect();
    if (!rect) return;
    const ctx = this._setupCanvas(canvas, rect);
    if (!ctx) return;
    const width = rect.width;
    const height = rect.height || this.canvasHeight;

    if (!this._processedSeries || this._processedSeries.length === 0) return;

    const { padding, graphWidth, graphHeight, minX, mapX, unmapX, unmapY } =
      this._getChartBoundsAndMapping(rect);
    if (minX === Infinity) return;

    // Draw Global Hover Sync Tracker
    if (!this._hoveredPoint && this.globalHoverX !== null) {
      const px = mapX(this.globalHoverX);
      if (px >= padding.left && px <= padding.left + graphWidth) {
        ctx.beginPath();
        ctx.strokeStyle = '#999';
        ctx.lineWidth = 0.5;
        ctx.setLineDash([4, 4]);
        ctx.moveTo(px, padding.top);
        ctx.lineTo(px, padding.top + graphHeight);
        ctx.stroke();
        ctx.setLineDash([]);
      }
    }

    // Draw Global Pinned Sync Tracker
    if (this.globalPinnedX !== null) {
      const px = mapX(this.globalPinnedX);
      if (px >= padding.left && px <= padding.left + graphWidth) {
        ctx.beginPath();
        ctx.strokeStyle = '#d93025'; // Google Red 600
        ctx.lineWidth = 1;
        ctx.setLineDash([2, 4]);
        ctx.moveTo(px, padding.top);
        ctx.lineTo(px, padding.top + graphHeight);
        ctx.stroke();
        ctx.setLineDash([]);
      }
    }

    // Draw Hover Overlays trackers
    const showCrosshair = this._hoveredPoint || (this._mousePos && !this._dragCtx.isDragging);
    if (showCrosshair) {
      const x = this._hoveredPoint ? this._hoveredPoint.x : this._mousePos!.x;
      const y = this._hoveredPoint ? this._hoveredPoint.y : this._mousePos!.y;

      // Only draw if inside graph area
      if (
        x >= padding.left &&
        x <= padding.left + graphWidth &&
        y >= padding.top &&
        y <= padding.top + graphHeight
      ) {
        ctx.beginPath();
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.4)';
        ctx.lineWidth = 0.5;
        ctx.setLineDash([4, 4]);

        // Horizontal crosshair
        ctx.moveTo(padding.left, y);
        ctx.lineTo(padding.left + graphWidth, y);

        // Vertical crosshair
        ctx.moveTo(x, padding.top);
        ctx.lineTo(x, padding.top + graphHeight);
        ctx.stroke();

        ctx.setLineDash([]); // Reset dash

        // Highlighted vertex marker
        if (this._hoveredPoint) {
          ctx.beginPath();
          ctx.fillStyle = this._hoveredPoint.series.color || '#1a73e8';
          ctx.arc(x, y, 4, 0, 2 * Math.PI);
          ctx.fill();

          ctx.beginPath();
          ctx.strokeStyle = '#fff';
          ctx.lineWidth = 1.5;
          ctx.arc(x, y, 4, 0, 2 * Math.PI);
          ctx.stroke();
        }

        // Draw dark chips on axes
        const value = this._hoveredPoint ? this._hoveredPoint.row.val : unmapY(y);
        const labelYStr = this._formatYValue(value);
        ctx.font = '10px "Inter", sans-serif';
        const textWidthY = ctx.measureText(labelYStr).width;
        ctx.fillStyle = 'rgba(0, 0, 0, 0.75)';
        const labelXPos = padding.left - textWidthY - 8;
        ctx.fillRect(labelXPos, y - 9, textWidthY + 6, 18);
        ctx.fillStyle = '#fff';
        ctx.textAlign = 'right';
        ctx.textBaseline = 'middle';
        ctx.fillText(labelYStr, padding.left - 5, y);

        const commit = this._hoveredPoint
          ? this._hoveredPoint.row.commit_number
          : Math.round(unmapX(x));
        const labelXStr = commit.toString();
        const textWidthX = ctx.measureText(labelXStr).width;
        ctx.fillStyle = 'rgba(0, 0, 0, 0.75)';
        const labelYPos = padding.top + graphHeight + 4;
        ctx.fillRect(x - textWidthX / 2 - 4, labelYPos, textWidthX + 8, 18);
        ctx.fillStyle = '#fff';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'top';
        ctx.fillText(labelXStr, x, labelYPos + 2);
      }
    }

    // Draw Measurement Overlay
    if (this._measureState) {
      const { startY, currentY, startDataY, currentDataY, currentX } = this._measureState;

      const boxStartY = Math.max(padding.top, Math.min(height - padding.bottom, startY));
      const boxCurrentY = Math.max(padding.top, Math.min(height - padding.bottom, currentY));

      const rectY = Math.min(boxStartY, boxCurrentY);
      const rectH = Math.abs(boxStartY - boxCurrentY);

      ctx.fillStyle = 'rgba(249, 171, 0, 0.1)';
      ctx.strokeStyle = 'rgba(249, 171, 0, 0.8)';
      ctx.lineWidth = 1;
      ctx.setLineDash([4, 4]);
      ctx.fillRect(padding.left, rectY, graphWidth, rectH);

      ctx.beginPath();
      ctx.moveTo(padding.left, rectY);
      ctx.lineTo(padding.left + graphWidth, rectY);
      ctx.moveTo(padding.left, rectY + rectH);
      ctx.lineTo(padding.left + graphWidth, rectY + rectH);
      ctx.stroke();
      ctx.setLineDash([]);

      const { diff, percent } = calculateMeasurement(startDataY, currentDataY);

      const isPctValid = isFinite(percent) && !isNaN(percent);
      const pctStr = isPctValid ? ` (${diff > 0 ? '+' : ''}${percent.toFixed(1)}%)` : '';
      const labelText = `ΔY: ${diff > 0 ? '+' : ''}${this._formatYValue(diff)}${pctStr}`;

      ctx.font = '12px sans-serif';
      ctx.fillStyle = 'rgba(255, 255, 255, 0.9)';
      const textWidth = ctx.measureText(labelText).width;

      let labelX = currentX + 10;
      let labelY = currentY + 20;

      if (labelX + textWidth + 10 > width) {
        labelX = currentX - textWidth - 10;
      }
      if (labelY + 20 > height) {
        labelY = currentY - 10;
      }

      // Draw text background
      ctx.fillStyle = 'rgba(15, 23, 42, 0.9)';
      ctx.fillRect(labelX - 4, labelY - 12, textWidth + 8, 16);
      ctx.strokeStyle = 'rgba(249, 171, 0, 0.8)';
      ctx.strokeRect(labelX - 4, labelY - 12, textWidth + 8, 16);

      ctx.fillStyle = 'rgba(255, 255, 255, 0.9)';
      ctx.fillText(labelText, labelX, labelY);
    }

    // Draw Range Selection Overlay
    if ((this._dragCtx.isDragging && this._dragCtx.isCtrl) || this._selectedRange) {
      const startX =
        this._dragCtx.isDragging && this._dragCtx.isCtrl
          ? this._dragCtx.dragStartX
          : this._selectedRange!.startX;
      const currentX =
        this._dragCtx.isDragging && this._dragCtx.isCtrl
          ? this._dragCtx.currentX
          : this._selectedRange!.endX;
      const rectX = Math.min(startX, currentX);
      const rectW = Math.abs(startX - currentX);

      ctx.fillStyle = 'rgba(26, 115, 232, 0.1)';
      ctx.strokeStyle = 'rgba(26, 115, 232, 0.8)';
      ctx.lineWidth = 1;
      ctx.fillRect(rectX, padding.top, rectW, graphHeight);
      ctx.strokeRect(rectX, padding.top, rectW, graphHeight);

      // Draw labels for selection
      const startVal = unmapX(startX);
      const currentVal = unmapX(currentX);

      const drawLabelX = (px: number, val: number) => {
        const label = Math.round(val).toString();
        ctx.font = '11px sans-serif';
        const textWidth = ctx.measureText(label).width;
        ctx.fillStyle = 'rgba(0, 0, 0, 0.75)';
        const labelY = padding.top + graphHeight + 4;
        ctx.fillRect(px - textWidth / 2 - 4, labelY, textWidth + 8, 18);
        ctx.fillStyle = '#fff';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'top';
        ctx.fillText(label, px, labelY + 2);
      };

      drawLabelX(startX, startVal);
      drawLabelX(currentX, currentVal);
    }
  }

  private _getChartBoundsAndMapping(rect: DOMRect) {
    let minX = Infinity,
      maxX = -Infinity,
      minY = Infinity,
      maxY = -Infinity,
      countMaxY = 0;
    let minTimestamp = Infinity,
      maxTimestamp = -Infinity;
    this._processedSeries.forEach((s) => {
      s.rows.forEach((r) => {
        const valX = this._xAccessor(r);
        const valY = Number(r.val);
        if (valX < minX) {
          minX = valX;
          minTimestamp = r.createdat;
        }
        if (valX > maxX) {
          maxX = valX;
          maxTimestamp = r.createdat;
        }
        if (valY < minY) minY = valY;
        if (valY > maxY) maxY = valY;
      });

      if (s.allStats && this.activeStats.has('min') && this.activeStats.has('max')) {
        ['min', 'max'].forEach((statKey) => {
          const rows = s.allStats![statKey];
          if (rows) {
            rows.forEach((r) => {
              const valX = this._xAccessor(r);
              // const valY = Number(r.val);
              if (valX < minX) {
                minX = valX;
                minTimestamp = r.createdat;
              }
              if (valX > maxX) {
                maxX = valX;
                maxTimestamp = r.createdat;
              }
              // Do not expand Y bounds for min/max traces to avoid flattening the graph
              // if (valY < minY) minY = valY;
              // if (valY > maxY) maxY = valY;
            });
          }
        });
      }

      if (s.allStats && this.activeStats.has('count')) {
        const countRows = s.allStats['count'];
        if (countRows) {
          countRows.forEach((r) => {
            const valY = Number(r.val);
            if (valY > countMaxY) countMaxY = valY;
          });
        }
      }
    });

    const displayMinX = this._viewportMinX ?? minX;
    const displayMaxX = this._viewportMaxX ?? maxX;

    const padding = this.isSparkline
      ? { top: 5, right: 5, bottom: 5, left: 5 }
      : { top: 10, right: 10, bottom: 32, left: computeLeftPadding(maxY, minY) };
    const graphWidth = rect.width - padding.left - padding.right;
    const graphHeight = rect.height - padding.top - padding.bottom;
    const yRange = maxY - minY || 1;

    let mapX: (val: number) => number;
    let unmapX: (px: number) => number;
    let xRange: number;

    if (this.evenXAxisSpacing && this._sortedXValues.length > 0) {
      const minIdx = getVirtualIndex(this._sortedXValues, displayMinX);
      const maxIdx = getVirtualIndex(this._sortedXValues, displayMaxX);
      xRange = maxIdx - minIdx || 1;

      mapX = (val: number) => {
        const idx = this._xValueToIndex.get(val);
        if (idx === undefined) return padding.left;
        return padding.left + ((idx - minIdx) / xRange) * graphWidth;
      };

      unmapX = (px: number) => {
        const idx = Math.round(minIdx + ((px - padding.left) / graphWidth) * xRange);
        const clampedIdx = Math.max(0, Math.min(idx, this._sortedXValues.length - 1));
        return this._sortedXValues[clampedIdx];
      };
    } else {
      xRange = displayMaxX - displayMinX || 1;
      mapX = (val: number) => padding.left + ((val - displayMinX) / xRange) * graphWidth;
      unmapX = (px: number) => displayMinX + ((px - padding.left) / graphWidth) * xRange;
    }

    const mapY = (val: number) => padding.top + graphHeight - ((val - minY) / yRange) * graphHeight;
    const unmapY = (py: number) => minY + ((padding.top + graphHeight - py) / graphHeight) * yRange;

    return {
      padding,
      graphWidth,
      graphHeight,
      minX: displayMinX,
      maxX: displayMaxX,
      minY,
      maxY,
      mapX,
      mapY,
      unmapX,
      unmapY,
      countMaxY,
      mapCountY: (val: number) =>
        padding.top + graphHeight - (val / (countMaxY * 1.1 || 1)) * graphHeight,
      globalMinX: minX,
      globalMaxX: maxX,
      minTimestamp,
      maxTimestamp,
    };
  }

  private _getVisibleDateRange() {
    let minTime = Infinity;
    let maxTime = -Infinity;

    let globalMinX = Infinity;
    let globalMaxX = -Infinity;
    this._processedSeries.forEach((s) =>
      s.rows.forEach((r) => {
        if (r.commit_number < globalMinX) globalMinX = r.commit_number;
        if (r.commit_number > globalMaxX) globalMaxX = r.commit_number;
      })
    );
    if (globalMinX === Infinity) return null;

    const viewMinX = this._viewportMinX !== null ? this._viewportMinX : globalMinX;
    const viewMaxX = this._viewportMaxX !== null ? this._viewportMaxX : globalMaxX;

    let maxVisibleX = -Infinity;

    this._processedSeries.forEach((s) => {
      s.rows.forEach((r) => {
        if (r.commit_number >= viewMinX && r.commit_number <= viewMaxX) {
          if (r.commit_number > maxVisibleX) maxVisibleX = r.commit_number;
          if (r.createdat > 0) {
            if (r.createdat < minTime) minTime = r.createdat;
            if (r.createdat > maxTime) maxTime = r.createdat;
          }
        }
      });
    });

    if (minTime === Infinity || maxTime === -Infinity) return null;

    const isAtGlobalMax = maxVisibleX >= globalMaxX;

    return { min: minTime, max: maxTime, isAtGlobalMax };
  }

  private _formatDate(ts: number): string {
    if (!ts || ts === Infinity || ts === -Infinity) return '';
    const date = ts > 1e11 ? new Date(ts) : new Date(ts * 1000);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(
      date.getDate()
    ).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(
      date.getMinutes()
    ).padStart(2, '0')}`;
  }

  private _handlePointerDown(e: PointerEvent) {
    const canvas = this.canvas;
    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;
    const mouseY = e.clientY - rect.top;

    const isShiftNow = e.shiftKey;
    const isCtrlNow = e.ctrlKey || e.metaKey;
    if (!isShiftNow && this._measureState) {
      this._measureState = null;
    }

    if (this._selectedRange) {
      this._selectedRange = null;
      this.dispatchEvent(
        new CustomEvent('range-cleared', {
          bubbles: true,
          composed: true,
        })
      );
    }

    this._dragCtx.isDragging = true;
    this._dragCtx.dragStartX = mouseX;
    this._dragCtx.dragStartY = mouseY;
    this._dragCtx.isShift = isShiftNow;
    this._dragCtx.isCtrl = isCtrlNow;
    this._dragCtx.currentX = mouseX;
    this._dragCtx.pointerId = e.pointerId;

    try {
      canvas.setPointerCapture(e.pointerId);
    } catch (_err) {
      /* ignore */
    }

    if (this._processedSeries.length === 0) return;

    const mapping = this._getChartBoundsAndMapping(rect);
    const startDataY = mapping.unmapY(mouseY);

    if (this._dragCtx.isShift) {
      this._measureState = {
        startX: mouseX,
        startY: mouseY,
        currentX: mouseX,
        currentY: mouseY,
        startDataY: startDataY,
        currentDataY: startDataY,
      };
    }
  }

  private _handlePointerMove(e: PointerEvent) {
    if (!this._processedSeries || this._processedSeries.length === 0) return;

    // If pinned and not dragging, don't update hover state so tooltip stays open.
    if (!this._dragCtx.isDragging && this.globalPinnedX !== null) {
      return;
    }

    const canvas = this.canvas;
    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;
    const mouseY = e.clientY - rect.top;

    const mapping = this._getChartBoundsAndMapping(rect);
    this._mousePos = { x: mouseX, y: mouseY };

    if (this._dragCtx.isDragging) {
      if (this._dragCtx.isShift) {
        const currentDataY = mapping.unmapY(mouseY);
        if (this._measureState) {
          this._measureState = {
            ...this._measureState,
            currentX: mouseX,
            currentY: mouseY,
            currentDataY: currentDataY,
          };
        }
      } else if (this._dragCtx.isCtrl) {
        this._dragCtx.currentX = mouseX;
        this._drawForeground();
      } else {
        const dx = mouseX - this._dragCtx.dragStartX;

        if (this.evenXAxisSpacing && this._sortedXValues.length > 0) {
          const vMinIdx = getVirtualIndex(this._sortedXValues, mapping.minX);
          const vMaxIdx = getVirtualIndex(this._sortedXValues, mapping.maxX);
          const vIdxRange = vMaxIdx - vMinIdx;
          let vIdxOffset = (dx / mapping.graphWidth) * vIdxRange;

          if (vMinIdx - vIdxOffset < 0) {
            vIdxOffset = vMinIdx;
          }
          if (vMaxIdx - vIdxOffset > this._sortedXValues.length - 1) {
            vIdxOffset = vMaxIdx - (this._sortedXValues.length - 1);
          }

          this._viewportMinX = getXFromVirtualIndex(this._sortedXValues, vMinIdx - vIdxOffset);
          this._viewportMaxX = getXFromVirtualIndex(this._sortedXValues, vMaxIdx - vIdxOffset);
        } else {
          const dataXOffset = (dx / mapping.graphWidth) * (mapping.maxX - mapping.minX);
          this._viewportMinX = mapping.minX - dataXOffset;
          this._viewportMaxX = mapping.maxX - dataXOffset;
        }

        this._dragCtx.dragStartX = mouseX;
        this._dragCtx.dragStartY = mouseY;

        this.requestUpdate();

        this.dispatchEvent(
          new CustomEvent('viewport-changed', {
            detail: {
              minCommit: this._viewportMinX,
              maxCommit: this._viewportMaxX,
            },
            bubbles: true,
            composed: true,
          })
        );
      }
      return;
    }

    // Hover logic
    let closestPoint: { series: TraceSeries; row: TraceRow; x: number; y: number } | null = null;
    let minDistanceSq = Infinity;

    const showOriginal = this.hoverMode === 'original' || this.hoverMode === 'both';
    const showSmoothed = this.hoverMode === 'smoothed' || this.hoverMode === 'both';

    const targetDataX =
      mapping.minX +
      ((mouseX - mapping.padding.left) / mapping.graphWidth) * (mapping.maxX - mapping.minX);

    for (const s of this._processedSeries) {
      if (s.rows.length === 0) continue;

      let low = 0;
      let high = s.rows.length - 1;
      while (low <= high) {
        const mid = Math.floor((low + high) / 2);
        if (this._xAccessor(s.rows[mid]) < targetDataX) {
          low = mid + 1;
        } else {
          high = mid - 1;
        }
      }

      for (let i = low; i < s.rows.length; i++) {
        const r = s.rows[i];
        const px = mapping.mapX(this._xAccessor(r));
        const dx = px - mouseX;
        if (dx * dx > minDistanceSq) break;

        if (showOriginal) {
          const py = mapping.mapY(r.val);
          const distSq = dx * dx + (py - mouseY) ** 2;
          if (distSq < minDistanceSq) {
            minDistanceSq = distSq;
            closestPoint = { series: s, row: r, x: px, y: py };
          }
        }

        if (showSmoothed && r.smoothedVal !== undefined) {
          const py = mapping.mapY(r.smoothedVal);
          const distSq = dx * dx + (py - mouseY) ** 2;
          if (distSq < minDistanceSq) {
            minDistanceSq = distSq;
            closestPoint = { series: s, row: r, x: px, y: py };
          }
        }
      }

      for (let i = low - 1; i >= 0; i--) {
        const r = s.rows[i];
        const px = mapping.mapX(this._xAccessor(r));
        const dx = px - mouseX;
        if (dx * dx > minDistanceSq) break;

        if (showOriginal) {
          const py = mapping.mapY(r.val);
          const distSq = dx * dx + (py - mouseY) ** 2;
          if (distSq < minDistanceSq) {
            minDistanceSq = distSq;
            closestPoint = { series: s, row: r, x: px, y: py };
          }
        }

        if (showSmoothed && r.smoothedVal !== undefined) {
          const py = mapping.mapY(r.smoothedVal);
          const distSq = dx * dx + (py - mouseY) ** 2;
          if (distSq < minDistanceSq) {
            minDistanceSq = distSq;
            closestPoint = { series: s, row: r, x: px, y: py };
          }
        }
      }
    }

    if (minDistanceSq < 900 && closestPoint) {
      this._hoveredPoint = closestPoint;
      this.dispatchEvent(
        new CustomEvent('hover-changed', {
          detail: { dataX: this._xAccessor(closestPoint.row) },
          bubbles: true,
          composed: true,
        })
      );
    } else {
      this._hoveredPoint = null;
      this.dispatchEvent(
        new CustomEvent('hover-changed', {
          detail: { dataX: null },
          bubbles: true,
          composed: true,
        })
      );
    }
  }

  private _handlePointerUp(e: PointerEvent) {
    const canvas = this.canvas;
    try {
      canvas.releasePointerCapture(e.pointerId);
    } catch (_err) {
      /* ignore */
    }

    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;
    const mouseY = e.clientY - rect.top;
    const dx = mouseX - this._dragCtx.dragStartX;
    const dy = mouseY - this._dragCtx.dragStartY;
    const dist = Math.sqrt(dx * dx + dy * dy);

    this._dragCtx.isDragging = false;

    if (this._dragCtx.isCtrl) {
      const startX = this._dragCtx.dragStartX;
      const currentX = mouseX;
      if (Math.abs(startX - currentX) > 5) {
        const mapping = this._getChartBoundsAndMapping(rect);
        const xRange = mapping.maxX - mapping.minX;
        const dataX1 =
          mapping.minX + ((startX - mapping.padding.left) / mapping.graphWidth) * xRange;
        const dataX2 =
          mapping.minX + ((currentX - mapping.padding.left) / mapping.graphWidth) * xRange;

        const newMin = Math.min(dataX1, dataX2);
        const newMax = Math.max(dataX1, dataX2);

        this._selectedRange = {
          minCommit: newMin,
          maxCommit: newMax,
          startX: startX,
          endX: currentX,
        };

        this.dispatchEvent(
          new CustomEvent('range-selected', {
            detail: {
              minCommit: newMin,
              maxCommit: newMax,
              startX: startX,
              endX: currentX,
              clientX: e.clientX,
              clientY: e.clientY,
            },
            bubbles: true,
            composed: true,
          })
        );
        return;
      }
    }

    if (dist < 5) {
      // Click detected
      if (this.globalPinnedX !== null) {
        // Clear pin if already pinned
        this.dispatchEvent(
          new CustomEvent('pin-point', {
            detail: { dataX: null },
            bubbles: true,
            composed: true,
          })
        );
      } else if (this._hoveredPoint) {
        // Pin it
        this.dispatchEvent(
          new CustomEvent('pin-point', {
            detail: { dataX: this._xAccessor(this._hoveredPoint.row) },
            bubbles: true,
            composed: true,
          })
        );
      }
    }
  }

  private _handlePointerLeave() {
    if (this._isMouseOverTooltip) {
      return;
    }
    this._hoveredPoint = null;
    this._mousePos = null;
  }

  private _handleWheel(e: WheelEvent) {
    if (!e.shiftKey) return;
    e.preventDefault();

    const canvas = this.canvas;
    if (!canvas) return;

    const rect = canvas.getBoundingClientRect();
    const { graphWidth, padding, minX, maxX, globalMinX, globalMaxX } =
      this._getChartBoundsAndMapping(rect);

    if (minX === Infinity || maxX === -Infinity) return;

    const mouseX = e.clientX - rect.left;
    const ratio = Math.max(0, Math.min(1, (mouseX - padding.left) / graphWidth));
    const cursorDataX = minX + ratio * (maxX - minX);

    const ZOOM_FACTOR = 0.001;
    const delta = e.deltaY || e.deltaX;
    const scale = Math.exp(delta * ZOOM_FACTOR);

    let newMin = cursorDataX - (cursorDataX - minX) * scale;
    let newMax = cursorDataX + (maxX - cursorDataX) * scale;

    if (globalMinX !== undefined && newMin < globalMinX) newMin = globalMinX;
    if (globalMaxX !== undefined && newMax > globalMaxX) newMax = globalMaxX;

    if (newMax - newMin < 5) return;

    this._viewportMinX = newMin;
    this._viewportMaxX = newMax;

    this.requestUpdate();

    this.dispatchEvent(
      new CustomEvent('viewport-changed', {
        detail: {
          minCommit: newMin,
          maxCommit: newMax,
        },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleDoubleClick() {
    this._viewportMinX = null;
    this._viewportMaxX = null;
    this.dispatchEvent(
      new CustomEvent('viewport-changed', {
        detail: { minCommit: null, maxCommit: null },
        bubbles: true,
        composed: true,
      })
    );
    this.requestUpdate();
  }

  static styles = css`
    :host {
      display: block;
      background: none;
      backdrop-filter: none;
      border: none;
      border-radius: 0;
      padding: 12px 0;
      margin-bottom: 16px;
      box-shadow: none;
      color: var(--on-surface, #f8fafc);
      border-bottom: 1px solid var(--outline, rgba(255, 255, 255, 0.05));
    }

    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
      padding-bottom: 8px;
      border-bottom: none;
    }

    .title {
      font-size: 12px;
      font-weight: 400;
      color: var(--on-surface-variant, #94a3b8);
      letter-spacing: -0.01em;
    }

    .canvas-container {
      position: relative;
      background: var(--background, #0b0f19);
      border: none;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      color: var(--on-surface, #64748b);
      font-size: 13px;
    }

    #overlay-canvas {
      position: absolute;
      top: 0;
      left: 0;
      pointer-events: none;
    }

    canvas {
      display: block;
      width: 100%;
      height: 100%;
    }

    .footer {
      display: flex;
      flex-direction: column;
      gap: 8px;
      margin-top: 12px;
      border-top: none;
      padding-top: 0;
      font-size: 12px;
    }

    .footer-row {
      display: flex;
      gap: 8px;
      align-items: center;
      flex-wrap: wrap;
    }

    .footer-label {
      color: var(--on-surface, #64748b);
      font-weight: 600;
      text-transform: uppercase;
      font-size: 10px;
      letter-spacing: 0.05em;
    }

    .chip {
      background: rgba(255, 255, 255, 0.05);
      border: none;
      color: var(--on-surface, #94a3b8);
      border-radius: 4px;
      padding: 2px 8px;
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 4px;
      transition: all 0.2s ease;
      font-family: monospace;
      font-size: 11px;
    }

    .chip:hover {
      background: rgba(255, 255, 255, 0.1);
      color: var(--on-background, #fff);
    }

    .chip.active {
      background: rgba(99, 102, 241, 0.2);
      color: var(--primary, #818cf8);
    }

    .chip.active:hover {
      background: rgba(99, 102, 241, 0.3);
    }

    .remove-icon {
      font-size: 14px;
      font-weight: bold;
      color: inherit;
      opacity: 0.7;
    }

    .remove-icon:hover {
      opacity: 1;
    }

    .loading-overlay {
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: rgba(15, 23, 42, 0.7);
      display: flex;
      flex-direction: column;
      justify-content: center;
      align-items: center;
      gap: 12px;
      color: var(--on-surface, #94a3b8);
      font-size: 14px;
      backdrop-filter: blur(2px);
      z-index: 10;
    }

    .spinner {
      width: 24px;
      height: 24px;
      border: 2px solid rgba(255, 255, 255, 0.1);
      border-radius: 50%;
      border-top-color: var(--primary, #818cf8);
      animation: spin 1s linear infinite;
    }

    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }
  `;

  render() {
    return html`
      <div class="header">
        <span class="title">${this.title || 'Untitled Chart'}</span>
        <div class="actions">
          <button
            style="border:none; background:none; cursor:pointer; color:#666;"
            @click=${() =>
              this.dispatchEvent(
                new CustomEvent('close-chart', { bubbles: true, composed: true })
              )}>
            &times;
          </button>
        </div>
      </div>
      <div
        class="canvas-container"
        style="height: ${this.canvasHeight}px;"
        @pointerdown=${this._handlePointerDown}
        @pointermove=${this._handlePointerMove}
        @pointerup=${this._handlePointerUp}
        @pointerleave=${this._handlePointerLeave}
        @wheel=${this._handleWheel}
        @dblclick=${this._handleDoubleClick}>
        <canvas id="chart-canvas" height="${this.canvasHeight}"></canvas>
        <canvas id="overlay-canvas" height="${this.canvasHeight}"></canvas>

        ${this.loading
          ? html`
              <div class="loading-overlay">
                <div class="spinner"></div>
                <span>Loading traces...</span>
              </div>
            `
          : ''}
        ${this._hoveredPoint && !this._dragCtx.isDragging && !this._selectedRange
          ? html`
              <trace-chart-tooltip-sk
                .hoveredPoint=${this._hoveredPoint}
                .dateMode=${this.dateMode}
                .yAxisLabel=${this.yAxisLabel}
                .regressions=${this.regressions}
                .diffNamesMap=${this._diffNamesMap}
                .tooltipDiffs=${this.tooltipDiffs}
                .processedSeries=${this._processedSeries}
                .showBisectButton=${this.show_bisect_button}
                .showPinpointButtons=${this._show_pinpoint_buttons}
                .canvasWidth=${this._canvasWidth}
                .canvasHeight=${this.canvasHeight}
                .user_id=${this.user_id}
                @pointerenter=${() => {
                  this._isMouseOverTooltip = true;
                }}
                @pointerleave=${() => {
                  this._isMouseOverTooltip = false;
                }}></trace-chart-tooltip-sk>
            `
          : ''}
      </div>

      ${(() => {
        const dateRange = this._getVisibleDateRange();
        if (!dateRange) return '';
        const isOld =
          dateRange.isAtGlobalMax && Date.now() - dateRange.max > 2 * 24 * 60 * 60 * 1000;
        return html`
          <div
            style="display: flex; justify-content: space-between; font-size: 12px; color: var(--on-surface-variant, #666); margin-top: 4px; padding-left: 60px; padding-right: 30px;">
            <span>${this._formatDate(dateRange.min)}</span>
            <span
              style="color: ${isOld ? '#d93025' : 'inherit'}; font-weight: ${isOld
                ? 'bold'
                : 'normal'};">
              ${this._formatDate(dateRange.max)}
            </span>
          </div>
        `;
      })()}

      <slot name="summary"></slot>

      ${this.activeSplitKeys.length > 0 || this._potentialSplitKeys.length > 0
        ? html`
            <div class="footer">
              <div class="footer-row">
                <span class="footer-label">Traces:</span>
                ${this._processedSeries.map(
                  (s) => html`
                    <div class="chip active" style="cursor: default;">
                      <span
                        style="display:inline-block; width:16px; height:2px; background:${s.color}; margin-right:6px;"></span>
                      ${this._diffNamesMap.get(s.id) || s.id}
                      <span
                        class="remove-icon"
                        style="cursor: pointer;"
                        @click=${() =>
                          this.dispatchEvent(
                            new CustomEvent('remove-trace', {
                              detail: { id: s.id },
                              bubbles: true,
                              composed: true,
                            })
                          )}
                        >&times;</span
                      >
                    </div>
                  `
                )}
              </div>
              ${this.activeSplitKeys.length > 0
                ? html`
                    <div class="footer-row">
                      <span class="footer-label">Current Splits:</span>
                      ${this.activeSplitKeys.map(
                        (key) => html`
                          <div
                            class="chip active"
                            draggable="true"
                            @dragstart=${(e: DragEvent) => this._handleDragKeyStart(e, key)}
                            @dragover=${(e: DragEvent) => this._handleDragKeyOver(e)}
                            @drop=${(e: DragEvent) => this._handleDropKey(e, key)}
                            @click=${() => this._toggleSplit(key)}>
                            ${key} <span class="remove-icon">&times;</span>
                          </div>
                        `
                      )}
                    </div>
                  `
                : ''}
              ${this._potentialSplitKeys.length > 0
                ? html`
                    <div class="footer-row">
                      <span class="footer-label">Split by:</span>
                      ${this._potentialSplitKeys.map(
                        (key) => html`
                          <div class="chip" @click=${() => this._toggleSplit(key)}>${key}</div>
                        `
                      )}
                    </div>
                  `
                : ''}
            </div>
          `
        : ''}
    `;
  }

  private _toggleSplit(key: string) {
    this.dispatchEvent(
      new CustomEvent('toggle-split', {
        detail: { key },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleDragKeyStart(e: DragEvent, key: string) {
    e.dataTransfer?.setData('text/plain', key);
  }

  private _handleDragKeyOver(e: DragEvent) {
    e.preventDefault();
  }

  private _handleDropKey(e: DragEvent, targetKey: string) {
    e.preventDefault();
    const draggedKey = e.dataTransfer?.getData('text/plain');
    if (!draggedKey || draggedKey === targetKey) return;

    const keys = [...this.activeSplitKeys];
    const draggedIdx = keys.indexOf(draggedKey);
    const targetIdx = keys.indexOf(targetKey);

    if (draggedIdx === -1 || targetIdx === -1) return;

    keys.splice(draggedIdx, 1);
    keys.splice(targetIdx, 0, draggedKey);

    this.dispatchEvent(
      new CustomEvent('reorder-split-keys', {
        detail: { keys },
        bubbles: true,
        composed: true,
      })
    );
  }
}
