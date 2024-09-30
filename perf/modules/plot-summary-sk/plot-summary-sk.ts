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
import { GoogleChart } from '@google-web-components/google-chart';
import { html, LitElement, PropertyValues } from 'lit';
import { ref, createRef } from 'lit/directives/ref.js';
import { define } from '../../../elements-sk/modules/define';
import { MousePosition, Point } from '../plot-simple-sk/plot-simple-sk';
import {
  ChartData,
  ConvertData,
  SummaryChartOptions,
} from '../common/plot-builder';
import { ColumnHeader } from '../json';
import { style } from './plot-summary-sk.css';
import { property } from 'lit/decorators.js';

const ZOOM_RECT_COLOR = '#0007';

// Describes the zoom in terms of x-axis source values.
export type ZoomRange = [number, number] | null;

export interface PlotSummarySkSelectionEventDetails {
  start: number;
  end: number;
  valueStart: number | Date;
  valueEnd: number | Date;
  domain: 'commit' | 'date';
}

export class PlotSummarySk extends LitElement {
  static styles = style;

  @property({ reflect: true })
  domain: 'commit' | 'date' = 'commit';

  constructor() {
    super();
    this.addEventListeners();
  }

  // The div element that will host the plot on the summary.
  private plotElement = createRef<GoogleChart>();

  protected render() {
    return html`
      <div class="container">
        <google-chart ${ref(this.plotElement)} class="plot" type="line">
        </google-chart>
        <canvas
          ${ref(this.overlayCanvas)}
          class="overlay"
          @mousedown=${(e: MouseEvent) => this.mouseDownListener(e)}
          @mousemove=${(e: MouseEvent) => this.mouseMoveListener(e)}
          @mouseleave=${() => this.mouseUpListener()}
          @mouseup=${() => this.mouseUpListener()}></canvas>
      </div>
    `;
  }

  connectedCallback() {
    super.connectedCallback();
    const resizeObserver = new ResizeObserver(
      (entries: ResizeObserverEntry[]) => {
        entries.forEach(() => {
          // The google chart needs to redraw when it is resized.
          this.plotElement.value?.redraw();
          this.requestUpdate();
        });
      }
    );
    resizeObserver.observe(this);
  }

  protected firstUpdated(_changedProperties: PropertyValues): void {
    if (!this.overlayCanvas.value) {
      return;
    }

    const resizeObserver = new ResizeObserver(
      (entries: ResizeObserverEntry[]) => {
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
      }
    );
    resizeObserver.observe(this.overlayCanvas.value!);

    // globalAlpha denotes the transparency of the fill. Setting this to 50%
    // allows us to show the highlight plus the portion of the plot highlighted.
    this.overlayCtx!.globalAlpha = 0.5;

    this.drawSummaryRect();
    requestAnimationFrame(() => this.raf());
  }

  // Select the provided range on the plot-summary.
  public Select(begin: ColumnHeader, end: ColumnHeader) {
    const isCommitScale = this.domain === 'commit';
    const col = isCommitScale ? 'offset' : 'timestamp';

    const chart = this.chartLayout;
    const startX = chart?.getXLocation(
      isCommitScale ? begin[col] : (new Date(begin[col] * 1000) as any)
    );
    const endX = chart?.getXLocation(
      isCommitScale ? end[col] : (new Date(end[col] * 1000) as any)
    );
    this.selectionRange = [startX || 0, endX || 0];
    this.drawSelection();
  }

  // Display the chart data on the plot.
  public DisplayChartData(chartData: ChartData, isCommitScale: boolean) {
    this.domain = isCommitScale ? 'commit' : 'date';

    this.plotElement.value!.data = ConvertData(chartData);
    this.plotElement.value!.options = SummaryChartOptions(
      getComputedStyle(this),
      this.domain
    );
    this.requestUpdate();
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
    return (
      chart &&
      (chart as google.visualization.CoreChartBase).getChartLayoutInterface()
    );
  }

  // Add all the event listeners.
  private addEventListeners(): void {
    // If the user toggles the theme to/from darkmode then redraw.
    document.addEventListener('theme-chooser-toggle', () => {
      // Update the options to trigger the redraw.
      if (this.plotElement.value) {
        this.plotElement.value!.options = SummaryChartOptions(
          getComputedStyle(this),
          this.domain
        );
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
      const leftDiff =
        this.currentMousePosition.clientX - this.selectionRange![0];
      const rightDiff =
        this.selectionRange![1] - this.currentMousePosition.clientX;
      this.lockedSelectionDiffs = [leftDiff, rightDiff];
    } else {
      // User is starting a new selection.
      this.selectionRange = [point.clientX, point.clientX + 0.1];
      this.isCurrentlySelecting = true;
    }
  }

  // Listener for the mouse up event.
  private mouseUpListener() {
    // Releasing the mouse means selection/dragging is done.
    this.isCurrentlySelecting = false;
    this.lockedSelectionDiffs = null;
    this.summarySelected();
  }

  private summarySelected() {
    if (this.selectionRange !== null) {
      const start =
        this.chartLayout?.getHAxisValue(this.selectionRange[0]) || 0;
      const end = this.chartLayout?.getHAxisValue(this.selectionRange[1]) || 0;
      this.dispatchEvent(
        new CustomEvent<PlotSummarySkSelectionEventDetails>(
          'summary_selected',
          {
            detail: {
              start: this.selectionRange[0],
              end: this.selectionRange[1],
              valueStart:
                this.domain === 'date'
                  ? (start as any).getTime() / 1000
                  : start,
              valueEnd:
                this.domain === 'date' ? (end as any).getTime() / 1000 : end,
              domain: this.domain,
            },
            bubbles: true,
          }
        )
      );
    }
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

  /** Mirrors the highlight_color attribute. */
  get highlightColor(): string {
    return this.getAttribute('highlight_color') || ZOOM_RECT_COLOR;
  }

  set highlightColor(val: string) {
    this.setAttribute('highlight_color', val);
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
      this.overlayCtx!.fillStyle = this.highlightColor;
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
      const isMovingOnLeft =
        Math.abs(currentx - startx) < Math.abs(currentx - endx);
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
