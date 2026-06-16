import { html, LitElement, css, PropertyValues } from 'lit';
import { property } from 'lit/decorators.js';
import { createRef, ref } from 'lit/directives/ref.js';
import { define } from '../../../elements-sk/modules/define';
import { HResizableBoxSk } from '../plot-summary-sk/h_resizable_box_sk';
import { TraceSeries, TraceRow } from './trace-types';
import '@material/web/iconbutton/outlined-icon-button.js';
import '@material/web/icon/icon.js';

// Ensure HResizableBoxSk is registered
if (!customElements.get('h-resizable-box-sk')) {
  define('h-resizable-box-sk', HResizableBoxSk);
}

/**
 * @module modules/explore-multi-v2-sk/plot-summary-v2-sk
 * @description Canvas rendering and decimation pipeline.
 */
export class PlotSummaryV2Sk extends LitElement {
  private canvasRef = createRef<HTMLCanvasElement>();

  private boxRef = createRef<HResizableBoxSk>();

  private containerRef = createRef<HTMLDivElement>();

  private resizeObserver: ResizeObserver | null = null;

  @property({ type: Array })
  series: TraceSeries[] = [];

  @property({ type: String })
  domain: 'commit' | 'date' = 'commit';

  @property({ type: Number })
  viewportMinX: number | null = null;

  @property({ type: Number })
  viewportMaxX: number | null = null;

  @property({ type: Boolean })
  evenXAxisSpacing = false;

  @property({ type: Boolean })
  loading = false;

  static styles = css`
    :host {
      display: block;
      margin-top: 12px;
    }

    .plot-summary-layout {
      display: flex;
      flex-direction: row;
      align-items: center;
      width: 100%;
      gap: 8px;
    }

    .summary-container {
      position: relative;
      flex: 1;
      height: 45px;
      border: 1px solid var(--outline, rgba(255, 255, 255, 0.1));
      border-radius: 6px;
      background: var(--background, #0b0f19);
      box-sizing: border-box;
      overflow: hidden;
    }

    md-outlined-icon-button {
      --md-outlined-icon-button-container-width: 32px;
      --md-outlined-icon-button-container-height: 32px;
      --md-outlined-icon-button-icon-size: 20px;
    }

    canvas {
      display: block;
      width: 100%;
      height: 100%;
    }

    h-resizable-box-sk {
      position: absolute;
      top: 0;
      bottom: 0;
      left: 0;
      width: 100%;
    }

    .overlay {
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: rgba(11, 15, 25, 0.5);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 10;
    }

    .spinner {
      width: 16px;
      height: 16px;
      border: 2px solid var(--primary, #1a73e8);
      border-top-color: transparent;
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }
  `;

  connectedCallback() {
    super.connectedCallback();
    this.resizeObserver = new ResizeObserver(() => {
      this.drawSummary();
      this.requestUpdate();
    });
    this.resizeObserver.observe(this);
  }

  disconnectedCallback() {
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
    }
    super.disconnectedCallback();
  }

  protected updated(changedProperties: PropertyValues) {
    super.updated(changedProperties);
    if (
      changedProperties.has('series') ||
      changedProperties.has('domain') ||
      changedProperties.has('evenXAxisSpacing')
    ) {
      this.drawSummary();
    }
  }

  private decimate(rows: TraceRow[]): TraceRow[] {
    const maxPoints = 500;
    if (rows.length <= maxPoints) {
      return rows;
    }
    const bucketSize = Math.ceil((2 * rows.length) / maxPoints);
    const decimated: TraceRow[] = [];

    for (let i = 0; i < rows.length; i += bucketSize) {
      const end = Math.min(i + bucketSize, rows.length);
      let minIdx = i;
      let maxIdx = i;

      for (let j = i + 1; j < end; j++) {
        if (rows[j].val < rows[minIdx].val) minIdx = j;
        if (rows[j].val > rows[maxIdx].val) maxIdx = j;
      }

      if (minIdx === maxIdx) {
        decimated.push(rows[minIdx]);
      } else if (minIdx < maxIdx) {
        decimated.push(rows[minIdx]);
        decimated.push(rows[maxIdx]);
      } else {
        decimated.push(rows[maxIdx]);
        decimated.push(rows[minIdx]);
      }
    }
    return decimated;
  }

  private getSeriesBounds(): { min: number; max: number } {
    const isDate = this.domain === 'date';
    const getX = (r: TraceRow) => (isDate ? r.createdat : r.commit_number);

    if (this.evenXAxisSpacing) {
      const uniqueX = new Set<number>();
      this.series
        .filter((s) => !s.hidden)
        .forEach((s) => {
          if (s.rows) {
            s.rows.forEach((r) => {
              const x = getX(r);
              if (x !== undefined) uniqueX.add(x);
            });
          }
        });
      const uniqueCount = uniqueX.size;
      return {
        min: 0,
        max: uniqueCount > 1 ? uniqueCount - 1 : 0,
      };
    }

    let minX = Infinity;
    let maxX = -Infinity;
    this.series
      .filter((s) => !s.hidden)
      .forEach((s) => {
        if (s.rows) {
          s.rows.forEach((r) => {
            const xVal = getX(r);
            if (xVal !== undefined) {
              if (xVal < minX) minX = xVal;
              if (xVal > maxX) maxX = xVal;
            }
          });
        }
      });

    return { min: minX, max: maxX };
  }

  private convertToCoordsRange(
    beginVal: number,
    endVal: number,
    width: number
  ): { begin: number; end: number } | null {
    const { min: minX, max: maxX } = this.getSeriesBounds();
    if (minX === Infinity || maxX === -Infinity || maxX === minX || width === 0) return null;

    const mapVal = (v: number) => ((v - minX) / (maxX - minX)) * width;
    return {
      begin: mapVal(beginVal),
      end: mapVal(endVal),
    };
  }

  private convertToValueRange(
    beginPx: number,
    endPx: number,
    width: number
  ): { begin: number; end: number } | null {
    const { min: minX, max: maxX } = this.getSeriesBounds();
    if (minX === Infinity || maxX === -Infinity || maxX === minX || width === 0) return null;

    const mapPx = (px: number) => minX + (px / width) * (maxX - minX);
    return {
      begin: mapPx(beginPx),
      end: mapPx(endPx),
    };
  }

  public drawSummary() {
    const canvas = this.canvasRef.value;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, rect.width, rect.height);

    if (!this.series || this.series.length === 0) return;

    const isDate = this.domain === 'date';
    const getX = (r: TraceRow) => (isDate ? r.createdat : r.commit_number);

    let sortedX: number[] = [];
    const xToIndex = new Map<number, number>();
    if (this.evenXAxisSpacing) {
      const uniqueX = new Set<number>();
      this.series.forEach((s) => {
        if (s.hidden) return;
        s.rows.forEach((r) => {
          uniqueX.add(getX(r));
        });
      });
      sortedX = Array.from(uniqueX).sort((a, b) => a - b);
      sortedX.forEach((v, i) => xToIndex.set(v, i));
    }

    let minX = Infinity;
    let maxX = -Infinity;
    let minY = Infinity;
    let maxY = -Infinity;

    this.series.forEach((s) => {
      if (s.hidden) return;
      s.rows.forEach((r) => {
        const xVal = this.evenXAxisSpacing ? xToIndex.get(getX(r))! : getX(r);
        if (xVal < minX) minX = xVal;
        if (xVal > maxX) maxX = xVal;
        if (r.val < minY) minY = r.val;
        if (r.val > maxY) maxY = r.val;
      });
    });

    if (minX === Infinity || minY === Infinity) return;

    const yDelta = maxY - minY;
    if (yDelta === 0) {
      minY -= 1;
      maxY += 1;
    } else {
      minY -= yDelta * 0.05;
      maxY += yDelta * 0.05;
    }

    const paddingY = 4;
    const drawableHeight = rect.height - 2 * paddingY;

    const mapX = (xVal: number) => {
      if (maxX === minX) return 0;
      return ((xVal - minX) / (maxX - minX)) * rect.width;
    };

    const mapY = (yVal: number) => {
      return rect.height - paddingY - ((yVal - minY) / (maxY - minY)) * drawableHeight;
    };

    ctx.lineWidth = 1.5;

    this.series.forEach((s) => {
      if (s.hidden) return;
      if (!s.rows || s.rows.length === 0) return;

      const decimated = this.decimate(s.rows);
      const color = s.color || '#1a73e8';
      ctx.strokeStyle = color;
      ctx.globalAlpha = 0.7;

      ctx.beginPath();
      decimated.forEach((r, idx) => {
        const xVal = this.evenXAxisSpacing ? xToIndex.get(getX(r))! : getX(r);
        const px = mapX(xVal);
        const py = mapY(r.val);
        if (idx === 0) {
          ctx.moveTo(px, py);
        } else {
          ctx.lineTo(px, py);
        }
      });
      ctx.stroke();
    });

    ctx.globalAlpha = 1.0;
  }

  protected render() {
    const parentWidth = this.containerRef.value?.offsetWidth || 0;
    let selectionRange = null;
    if (this.viewportMinX !== null && this.viewportMaxX !== null) {
      const coords = this.convertToCoordsRange(this.viewportMinX, this.viewportMaxX, parentWidth);
      if (coords) {
        selectionRange = { begin: coords.begin, end: coords.end };
      }
    }

    return html`
      <div class="plot-summary-layout">
        <md-outlined-icon-button ?disabled=${this.loading} @click=${() => this.loadMore('left')}>
          <md-icon>chevron_left</md-icon>
        </md-outlined-icon-button>
        <div class="summary-container" ${ref(this.containerRef)}>
          <canvas ${ref(this.canvasRef)}></canvas>
          <h-resizable-box-sk
            ${ref(this.boxRef)}
            .selectionRange=${selectionRange}
            @selection-changed=${this.handleBoxChanged}>
          </h-resizable-box-sk>
          ${this.loading
            ? html`
                <div class="overlay">
                  <div class="spinner"></div>
                </div>
              `
            : ''}
        </div>
        <md-outlined-icon-button ?disabled=${this.loading} @click=${() => this.loadMore('right')}>
          <md-icon>chevron_right</md-icon>
        </md-outlined-icon-button>
      </div>
    `;
  }

  private loadMore(side: 'left' | 'right') {
    this.dispatchEvent(
      new CustomEvent('load-more-click', {
        detail: side,
        bubbles: true,
        composed: true,
      })
    );
  }

  private handleBoxChanged(e: CustomEvent<{ begin: number; end: number } | null>) {
    const detail = e.detail;
    if (!detail) {
      const { min: minX, max: maxX } = this.getSeriesBounds();
      this.dispatchEvent(
        new CustomEvent('summary-range-selected', {
          detail: { begin: minX, end: maxX },
          bubbles: true,
          composed: true,
        })
      );
      return;
    }

    const parentWidth = this.containerRef.value?.offsetWidth || 0;
    const valueRange = this.convertToValueRange(detail.begin, detail.end, parentWidth);
    if (valueRange) {
      this.dispatchEvent(
        new CustomEvent('summary-range-selected', {
          detail: {
            begin: valueRange.begin,
            end: valueRange.end,
          },
          bubbles: true,
          composed: true,
        })
      );
    }
  }

  /**
   * Select programmatic positioning skeletal interface.
   */
  Select(begin: number, end: number) {
    console.log('Programmatic skeletal Select called:', begin, end);
  }
}

define('plot-summary-v2-sk', PlotSummaryV2Sk);
