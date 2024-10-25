/**
 * @module modules/plot-summary-sk
 * @description <h2><code>plot-summary-sk</code></h2>
 *
 * @evt summary_selected - Event produced when the user selects a section on the summary.
 *
 * @attr
 *
 * @example
 */
import '@google-web-components/google-chart';
import '@material/web/iconbutton/icon-button';
import '../dataframe/dataframe_context';

import { GoogleChart } from '@google-web-components/google-chart';
import { MdIconButton } from '@material/web/iconbutton/icon-button.js';
import { consume } from '@lit/context';
import { html, LitElement, PropertyValues } from 'lit';
import { ref, createRef } from 'lit/directives/ref.js';
import { property } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';

import { SummaryChartOptions } from '../common/plot-builder';
import { ColumnHeader } from '../json';
import { MousePosition, Point } from '../plot-simple-sk/plot-simple-sk';
import {
  dataframeLoadingContext,
  dataframeRepoContext,
  DataFrameRepository,
  DataTable,
  dataTableContext,
} from '../dataframe/dataframe_context';
import { range } from '../dataframe/index';
import { define } from '../../../elements-sk/modules/define';

import { style } from './plot-summary-sk.css';
import { HResizableBoxSk } from './h_resizable_box_sk';

// Describes the zoom in terms of x-axis source values.
export type ZoomRange = [number, number] | null;

export interface PlotSummarySkSelectionEventDetails {
  start: number;
  end: number;
  value: range;
  domain: 'commit' | 'date';
}

const dayInSeconds = 60 * 60 * 24;

export class PlotSummarySk extends LitElement {
  static styles = style;

  @consume({ context: dataframeRepoContext })
  private dfRepo?: DataFrameRepository;

  @consume({ context: dataframeLoadingContext, subscribe: true })
  @property({ reflect: true, type: Boolean })
  private loading = false;

  @property({ reflect: true })
  domain: 'commit' | 'date' = 'commit';

  @consume({ context: dataTableContext, subscribe: true })
  @property({ attribute: false })
  data: DataTable = null;

  @property({ reflect: true })
  selectionType: 'canvas' | 'material' = 'canvas';

  @property({ attribute: true })
  selectedTrace: string | null = null;

  @property({ type: Number })
  loadingChunk = 45 * dayInSeconds;

  @property({ type: Boolean })
  hasControl: boolean = false;

  constructor() {
    super();
    this.addEventListeners();
  }

  protected willUpdate(changedProperties: PropertyValues): void {
    if (
      changedProperties.has('data') ||
      changedProperties.has('selectedTrace') ||
      changedProperties.has('domain')
    ) {
      this.updateDataView(this.data, this.selectedTrace);
    }
  }

  private async updateDataView(dt: DataTable, trace: string | null) {
    await this.updateComplete;
    const plot = this.plotElement.value;
    if (!plot || !dt) {
      if (dt) {
        console.warn(
          'The dataframe is not assigned because the element is not ready. Try call `await this.updateComplete` first.'
        );
      }
      return;
    }

    const view = new google.visualization.DataView(dt!);
    const ncols = view.getNumberOfColumns();

    // The first two columns are the commit position and the date.
    const cols = [this.domain === 'commit' ? 0 : 1];
    for (let index = 2; index < ncols; index++) {
      const traceKey = view.getColumnLabel(index);
      if (!trace || trace === traceKey) {
        cols.push(index);
      }
    }

    view.setColumns(cols);
    plot.view = view;
    plot.options = SummaryChartOptions(getComputedStyle(this), this.domain);
  }

  // The div element that will host the plot on the summary.
  private plotElement = createRef<GoogleChart>();

  // The resizable selection box to draw the selection.
  private selectionBox = createRef<HResizableBoxSk>();

  // The current selected value saved for re-adjusting selection box.
  // The google chart reloads and redraws themselves asynchronously, we need to
  // save the selection range and apply it after we received the `ready` event.
  private cachedSelectedValueRange: range | null = null;

  private controlTemplate(side: 'right' | 'left') {
    const chunk = this.loadingChunk;
    const direction = side === 'left' ? -1 : 1;

    const onClickLoad = async ({ target }: Event) => {
      const btn = target as MdIconButton;
      await this.dfRepo?.extendRange(chunk * direction);
      btn.selected = false;
    };

    return html`
      ${when(
        this.hasControl && this.dfRepo,
        () =>
          html` <md-icon-button
            toggle
            ?disabled="${this.loading}"
            class="load-btn"
            @click=${onClickLoad}>
            <md-icon>keyboard_double_arrow_${side}</md-icon>
            <div slot="selected" class="loader"></div>
          </md-icon-button>`
      )}
    `;
  }

  protected render() {
    return html`
      ${this.controlTemplate('left')}
      <div class="container hover-to-show-link">
        <google-chart
          ${ref(this.plotElement)}
          class="plot"
          type="area"
          @google-chart-ready=${this.onGoogleChartReady}>
        </google-chart>
        ${when(
          this.selectionType === 'canvas',
          () =>
            html`<canvas
              ${ref(this.overlayCanvas)}
              class="overlay"
              @mousedown=${this.mouseDownListener}
              @mousemove=${this.mouseMoveListener}
              @mouseleave=${this.mouseUpListener}
              @mouseup=${this.mouseUpListener}>
            </canvas>`,
          () =>
            html`<h-resizable-box-sk
              ${ref(this.selectionBox)}
              @selection-changed=${this.onSelectionChanged}></h-resizable-box-sk>`
        )}
      </div>
      ${this.controlTemplate('right')}
    `;
  }

  connectedCallback() {
    super.connectedCallback();
    const resizeObserver = new ResizeObserver((entries: ResizeObserverEntry[]) => {
      entries.forEach(() => {
        // The google chart needs to redraw when it is resized.
        this.plotElement.value?.redraw();
        this.requestUpdate();
      });
    });
    resizeObserver.observe(this);
  }

  private onGoogleChartReady() {
    // Update the selectionBox because the chart might get updated.
    if (!this.cachedSelectedValueRange) {
      this.clearSelection();
      return;
    }
    this.selectedValueRange = this.cachedSelectedValueRange;
  }

  protected firstUpdated(_changedProperties: PropertyValues): void {
    if (!this.overlayCanvas.value) {
      return;
    }

    const resizeObserver = new ResizeObserver((entries: ResizeObserverEntry[]) => {
      entries.forEach((entry) => {
        if (entry.target !== this.overlayCanvas.value) {
          return;
        }

        // We need to resize the canvas after its bounding rect is changed,
        const boundingRect = entry.contentRect;
        this.overlayCanvas.value!.width = boundingRect!.width;
        this.overlayCanvas.value!.height = boundingRect!.height;
        this.overlayCtx!.globalAlpha = 0.5;
      });
    });
    resizeObserver.observe(this.overlayCanvas.value!);

    // globalAlpha denotes the transparency of the fill. Setting this to 50%
    // allows us to show the highlight plus the portion of the plot highlighted.
    this.overlayCtx!.globalAlpha = 0.5;

    this.drawSummaryRect();
    requestAnimationFrame(() => this.raf());
  }

  private onSelectionChanged(e: CustomEvent) {
    const valueRange = this.convertToValueRange(e.detail, this.domain) || range(0, 0);
    this.cachedSelectedValueRange = valueRange;
    this.dispatchEvent(
      new CustomEvent<PlotSummarySkSelectionEventDetails>('summary_selected', {
        detail: { start: 0, end: 0, value: valueRange, domain: this.domain },
        bubbles: true,
      })
    );
  }

  // Converts from the value range to the coordinates range.
  private convertToCoordsRange(valueRange: range | null, domain: 'date' | 'commit') {
    const chart = this.chartLayout;
    if (!chart || !valueRange) {
      return null;
    }
    const isCommitScale = domain === 'commit';
    const startX = chart?.getXLocation(
      isCommitScale ? valueRange.begin : (new Date(valueRange.begin * 1000) as any)
    );
    const endX = chart?.getXLocation(
      isCommitScale ? valueRange.end : (new Date(valueRange.end * 1000) as any)
    );

    return range(startX, endX);
  }

  // Converts from the value range to the coordinates range.
  private convertToValueRange(coordsRange: range | null, domain: 'date' | 'commit') {
    const chart = this.chartLayout;
    if (!chart || !coordsRange) {
      return null;
    }
    const range: range = {
      begin: chart.getHAxisValue(coordsRange.begin),
      end: chart.getHAxisValue(coordsRange.end),
    };
    // The date is saved in Date, we need to convert to the UNIX timestamp.
    if (domain === 'date') {
      range.begin = (range.begin as any).getTime() / 1000;
      range.end = (range.end as any).getTime() / 1000;
    }
    return range;
  }

  // Get the current selected value range.
  get selectedValueRange(): range | null {
    const chart = this.chartLayout;
    if (!chart) {
      return { begin: 0, end: 0 };
    }

    const coordsRange =
      this.selectionBox.value?.selectionRange ??
      (this.selectionRange ? range(this.selectionRange![0], this.selectionRange![1]) : null);
    const valueRange = this.convertToValueRange(coordsRange, this.domain);
    if (valueRange) {
      return valueRange;
    } else {
      return null;
    }
  }

  // Set the current selected value range.
  set selectedValueRange(range: range | null) {
    this.cachedSelectedValueRange = range;

    const chartRange = this.convertToCoordsRange(range, this.domain);
    const box = this.selectionBox.value;
    if (box) {
      box.selectionRange = chartRange;
    }

    // Legace canvas selection.
    if (chartRange) {
      this.selectionRange = [chartRange.begin, chartRange.end];
      this.drawSelection();
    } else {
      this.clearSelection();
    }
  }

  // Select the provided range on the plot-summary.
  public Select(begin: ColumnHeader, end: ColumnHeader) {
    const isCommitScale = this.domain === 'commit';
    const col = isCommitScale ? 'offset' : 'timestamp';
    this.selectedValueRange = { begin: begin[col], end: end[col] };
  }

  // Get the underlying ChartLayoutInterface.
  // This provides API to inspect the traces and coordinates.
  private get chartLayout(): google.visualization.ChartLayoutInterface | null {
    const gchart = this.plotElement.value;
    if (!gchart) {
      return null;
    }
    const wrapper = gchart['chartWrapper'] as google.visualization.ChartWrapper;
    if (!wrapper) {
      return null;
    }
    const chart = wrapper.getChart();
    return chart && (chart as google.visualization.CoreChartBase).getChartLayoutInterface();
  }

  // Add all the event listeners.
  private addEventListeners(): void {
    // If the user toggles the theme to/from darkmode then redraw.
    document.addEventListener('theme-chooser-toggle', () => {
      // Update the options to trigger the redraw.
      if (this.plotElement.value && this.data) {
        this.plotElement.value!.options = SummaryChartOptions(getComputedStyle(this), this.domain);
      }
      this.requestUpdate();
    });
  }

  // ======== Selection using Canvas =========
  // Below is using the canvas to track and draw user selections.
  // This gives us ability to draw our own stylish UIs and also requires to
  // manage user interaction ourselves. The canvas does require a bit more
  // frequent update and draws than native HTML elements w/o optimizations.
  private get overlayCtx() {
    return this.overlayCanvas.value?.getContext('2d');
  }

  // Array denoting the start and end points of the current selection.
  private selectionRange: ZoomRange = null;

  // Array denoting the diffs between the currently clicked point and
  // the start and end points of the current selection. This is used
  // to maintain the size of the selection when user drags it left/right.
  private lockedSelectionDiffs: [number, number] | null = null;

  // Denotes if the user is currently making a selection.
  private isCurrentlySelecting: boolean = false;

  // Tracks the mouse position.
  private currentMousePosition: MousePosition | null = null;

  // The canvas element that is used for the selection overlay.
  private overlayCanvas = createRef<HTMLCanvasElement>();

  // Clear the current selection.
  private clearSelection(): void {
    this.overlayCtx?.clearRect(
      0,
      0,
      this.overlayCanvas.value?.width || 0,
      this.overlayCanvas.value?.height || 0
    );
  }

  // Listener for the mouse down event.
  private mouseDownListener(e: MouseEvent) {
    e.preventDefault();
    const point = this.eventToCanvasPt(e);

    this.currentMousePosition = point;

    if (this.inSelectedArea({ x: point.clientX, y: point.clientY })) {
      // Implement the drag functionality to drag the selected area around.
      // This means the user is dragging the current selection around.
      // Let's get the left and right diffs between the current mouse position
      // and the ends of the selection range. As the mouse moves, we will update
      // the selection range so that these diffs remain constant.
      const leftDiff = this.currentMousePosition.clientX - this.selectionRange![0];
      const rightDiff = this.selectionRange![1] - this.currentMousePosition.clientX;
      this.lockedSelectionDiffs = [leftDiff, rightDiff];
    } else {
      // User is starting a new selection.
      this.selectionRange = [point.clientX, point.clientX + 0.1];
      this.isCurrentlySelecting = true;
    }
  }

  // Listener for the mouse up event.
  private mouseUpListener() {
    if (this.isCurrentlySelecting || this.lockedSelectionDiffs !== null) {
      // Releasing the mouse means selection/dragging is done.
      this.isCurrentlySelecting = false;
      this.lockedSelectionDiffs = null;
      this.summarySelected();
    }
  }

  private summarySelected() {
    if (!this.selectionRange) {
      return;
    }
    this.dispatchEvent(
      new CustomEvent<PlotSummarySkSelectionEventDetails>('summary_selected', {
        detail: {
          start: this.selectionRange[0],
          end: this.selectionRange[1],
          value:
            this.convertToValueRange(
              range(this.selectionRange[0], this.selectionRange[1]),
              this.domain
            ) || range(0, 0),
          domain: this.domain,
        },
        bubbles: true,
      })
    );
  }

  // Listener for the mouse move event.
  private mouseMoveListener(e: MouseEvent) {
    // Keep track of the user's mouse movements.
    const point = this.eventToCanvasPt(e);
    this.currentMousePosition = point;
  }

  // Draw the summary rectangle outline.
  private drawSummaryRect(): void {
    if (!this.overlayCanvas) {
      return;
    }
    const style = getComputedStyle(this);
    this.overlayCtx!.lineWidth = 3.0;
    this.overlayCtx!.strokeStyle = style.color;
    this.overlayCtx!.strokeRect(0, 0, this.width, this.height);
    this.overlayCtx!.save();
  }

  /** Mirrors the width attribute. */
  get width(): number {
    return this.getBoundingClientRect().width;
  }

  /** Mirrors the height attribute. */
  get height(): number {
    return this.getBoundingClientRect().height;
  }

  // Converts an event to a specific point
  private eventToCanvasPt(e: MousePosition) {
    const clientRect = this.plotElement.value!.getBoundingClientRect();
    return {
      clientX: e.clientX - clientRect!.left,
      clientY: e.clientY - clientRect!.top,
    };
  }

  // Draws the selection area.
  private drawSelection(): void {
    this.clearSelection();
    if (this.overlayCanvas.value && this.selectionRange !== null) {
      // Draw left line.
      const startx = this.selectionRange[0];
      this.drawVerticalLineAtPosition(startx);
      // Draw the right line.
      const endx = this.selectionRange[1];
      this.drawVerticalLineAtPosition(endx);

      // Shade the selected section.
      this.overlayCtx!.beginPath();
      this.overlayCtx!.fillStyle =
        getComputedStyle(this).getPropertyValue('--sk-summary-highlight');
      this.overlayCtx!.rect(startx, 0, endx - startx, this.height);
      this.overlayCtx!.fill();
    }
  }

  // Draw a vertical line at the given position
  private drawVerticalLineAtPosition(x: number) {
    // Start at the bottom of the rectangle
    this.overlayCtx!.moveTo(x, 0);
    this.overlayCtx!.lineTo(x, this.height);
    this.overlayCtx!.stroke();
  }

  // Handles the animation depending on the current state.
  private raf() {
    // Always queue up our next raf first.
    window.requestAnimationFrame(() => this.raf());

    // Exit early if there is no ongoing activity.
    if (this.currentMousePosition === null) {
      return;
    }

    if (this.isCurrentlySelecting) {
      // If the user is currently selecting an area, update the selection range
      // array based on the current mouse position.
      let startx = this.selectionRange![0];
      let endx = this.selectionRange![1];
      const currentx = this.currentMousePosition.clientX;

      // Figure out the closest end of the selection to the current position.
      // This tells us the direction in which the user is highlighting.
      const isMovingOnLeft = Math.abs(currentx - startx) < Math.abs(currentx - endx);
      if (isMovingOnLeft) {
        // If the mouse is towards the left side, we update the start values.
        startx = currentx;
      } else {
        // If the mouse is towards the right, we update the end values.
        endx = currentx;
      }
      this.selectionRange![0] = startx;
      this.selectionRange![1] = endx;
    } else if (this.lockedSelectionDiffs !== null) {
      // User is dragging the current selection around.
      // Update the selectionRange with the values that keeps these
      // diffs constant wrt the current mouse position. Also account for the fact
      // that the user may drag the selected area beyond the bounds of the rendered rect.
      this.selectionRange![0] = Math.max(
        0,
        this.currentMousePosition.clientX - this.lockedSelectionDiffs![0]
      );
      this.selectionRange![1] = Math.min(
        this.width,
        this.currentMousePosition.clientX + this.lockedSelectionDiffs![1]
      );
    }

    this.drawSelection();
  }

  // Checks if a point is inside the selected area.
  private inSelectedArea(pt: Point): boolean {
    if (this.selectionRange === null) {
      return false;
    }

    return pt.x >= this.selectionRange[0] && pt.x < this.selectionRange[1];
  }
}

define('plot-summary-sk', PlotSummarySk);

declare global {
  interface HTMLElementTagNameMap {
    'plot-summary-sk': PlotSummarySk;
  }

  interface GlobalEventHandlersEventMap {
    summary_selected: CustomEvent<PlotSummarySkSelectionEventDetails>;
  }
}
