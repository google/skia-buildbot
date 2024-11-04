/**
 * @module modules/plot-google-chart-sk
 * @description <h2><code>plot-google-chart-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import '@google-web-components/google-chart';
import '@material/web/progress/linear-progress';
import { GoogleChart } from '@google-web-components/google-chart';

import { consume } from '@lit/context';
import { html, css } from 'lit';
import { LitElement, PropertyValues } from 'lit';
import { ref, Ref, createRef } from 'lit/directives/ref.js';
import { property } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import { define } from '../../../elements-sk/modules/define';
import { Anomaly, AnomalyMap, DataFrame } from '../json';
import { convertFromDataframe, mainChartOptions } from '../common/plot-builder';
import {
  dataframeAnomalyContext,
  dataframeContext,
  dataframeLoadingContext,
} from '../dataframe/dataframe_context';
import { getTitle, titleFormatter, isSingleTrace } from '../dataframe/traceset';
import { range } from '../dataframe/index';
import { VResizableBoxSk } from './v-resizable-box-sk';
import { SidePanelSk } from './side-panel-sk';

export interface AnomalyData {
  x: number;
  y: number;
  anomaly: Anomaly;
  highlight: boolean;
}

export interface PlotSelectionEventDetails {
  value: range;
  domain: 'commit' | 'date';
}

export class PlotGoogleChartSk extends LitElement {
  // TODO(b/362831653): Adjust height to 100% once plot-summary-sk is deprecated
  static styles = css`
    :host {
      background-color: var(--plot-background-color-sk, var(--md-sys-color-background, 'white'));
    }
    slot {
      display: none;
    }
    .container {
      display: flex;
      height: 100%;
    }
    .side {
      max-width: 20%;
    }
    .plot {
      height: 100%;
      flex-grow: 1;
    }
    .anomaly {
      position: absolute;
      top: 0px;
      left: 0px;

      md-icon {
        transform: translate(-50%, -50%);
        border-radius: 50%;
        pointer-events: none;
        font-weight: bolder;
        font-size: 24px;
        color: darkblue;
      }
      md-icon.improvement {
        background-color: rgba(3, 151, 3, 1);
        font-weight: bolder;
        border: black 1px solid;
      }
      md-icon.untriage {
        background-color: yellow;
        border: magenta 1px solid;
      }
      md-icon.regression {
        background-color: violet;
        border: orange 1px solid;
      }
    }

    .delta {
      position: absolute;
      border: 1px solid purple;
      background-color: rgba(255, 255, 0, 0.2); /* semi-transparent yellow */
      top: 0px;

      p {
        position: relative;
        top: 10px;
        left: 10px;
      }
    }

    md-linear-progress {
      position: absolute;
      width: 100%;
      --md-linear-progress-active-indicator-height: 8px;
      --md-linear-progress-track-height: 8px;
      --md-linear-progress-track-shape: 8px;
    }
  `;

  @consume({ context: dataframeLoadingContext, subscribe: true })
  private loading = false;

  @consume({ context: dataframeContext, subscribe: true })
  @property({ attribute: false })
  private dataframe?: DataFrame;

  @property({})
  selectedTraces: string[] | null = null;

  @property({ reflect: true })
  domain: 'commit' | 'date' = 'commit';

  @property({ attribute: false })
  selectedRange?: range;

  @consume({ context: dataframeAnomalyContext, subscribe: true })
  @property({ attribute: false })
  private anomalyMap: AnomalyMap = {};

  // The slots to place in the templated icons for anomalies.
  private slots = {
    untriage: createRef<HTMLSlotElement>(),
    regression: createRef<HTMLSlotElement>(),
    improvement: createRef<HTMLSlotElement>(),
  };

  // Modes for chart interaction with mouse.
  // We only have panning and deltaY for now.
  // Default behavior is null.
  // - panning (enabled by dragging) pans the chart to the left or right
  // - deltaY (enabled with shift-click) calculates the delta on the
  // y-axis between the start and end cursor.
  @property({ attribute: false })
  private navigationMode: 'pan' | 'deltaY' | null = null;

  private lastMouse = { x: 0, y: 0 };

  // The value distance when moving by 1px on the screen.
  private valueDelta = 1;

  private cachedChartArea = { left: 0, top: 0, width: 0, height: 0 };

  // Whether we are interacting with the chart that takes higher prioritiy than navigations.
  private chartInteracting = false;

  // cache the googleChart object within the module
  private chart: google.visualization.CoreChartBase | null = null;

  constructor() {
    super();

    this.addEventListeners();
  }

  // The div element that will host the plot on the summary.
  private plotElement: Ref<GoogleChart> = createRef();

  // The div container for anomaly overlays.
  private anomalyDiv = createRef<HTMLDivElement>();

  // The div container for delta y selection range.
  private deltaRangeBox = createRef<VResizableBoxSk>();

  // The div container for the legend
  private sidePanel = createRef<SidePanelSk>();

  connectedCallback(): void {
    super.connectedCallback();

    const resizeObserver = new ResizeObserver((entries: ResizeObserverEntry[]) => {
      entries.forEach(() => {
        // The google chart needs to redraw when it is resized.
        this.plotElement.value?.redraw();
      });
    });
    resizeObserver.observe(this);
  }

  protected render() {
    // TODO(b/370804498): Break out plot panel into a separate module
    // and create a new module that combines google chart and the
    // tooltip module.
    // TODO(b/370804689): Add legend to side panel
    return html`
      <div class="container">
        <google-chart
          ${ref(this.plotElement)}
          class="plot"
          type="line"
          .events=${['onmouseover', 'onmouseout']}
          @mousedown=${this.onChartMouseDown}
          @mouseup=${this.onChartMouseUp}
          @google-chart-onmouseover=${this.onChartMouseOver}
          @google-chart-onmouseout=${this.onChartMouseOut}
          @google-chart-ready=${this.onChartReady}>
        </google-chart>
        ${when(this.loading, () => html`<md-linear-progress indeterminate></md-linear-progress>`)}
        <div class="anomaly" ${ref(this.anomalyDiv)}></div>
        <v-resizable-box-sk ${ref(this.deltaRangeBox)}} @mouseup=${this.onChartMouseUp}>
        </v-resizable-box-sk>
        <div class="side" ?hidden=${isSingleTrace(this.dataframe) ?? true}>
          <side-panel-sk ${ref(this.sidePanel)} @side-panel-toggle=${this.onSidePanelToggle}>
          </side-panel-sk>
        </div>
      </div>
      <slot name="untriage" ${ref(this.slots.untriage)}></slot>
      <slot name="regression" ${ref(this.slots.regression)}></slot>
      <slot name="improvement" ${ref(this.slots.improvement)}></slot>
    `;
  }

  protected willUpdate(changedProperties: PropertyValues): void {
    // TODO(b/362831653): incorporate domain changes into dataframe update.
    if (changedProperties.has('dataframe')) {
      this.updateDataframe(this.dataframe!);
    } else if (changedProperties.has('anomalyMap')) {
      // If the anomalyMap is getting updated,
      // trigger the chart to redraw and plot the anomaly.
      this.plotElement.value?.redraw();
    } else if (changedProperties.has('selectedRange')) {
      // If only the selectedRange is updated, then we only update the viewWindow.
      this.updateOptions();
    }
  }

  private updateDataframe(df: DataFrame) {
    const rows = convertFromDataframe(df, this.domain);
    if (!rows) {
      return;
    }
    this.plotElement.value!.data = rows;
    this.updateOptions();
  }

  private updateOptions() {
    const options = mainChartOptions(
      getComputedStyle(this),
      this.domain,
      titleFormatter(getTitle(this.dataframe!))
    );
    options.hAxis!.viewWindow = {
      min: this.selectedRange?.begin,
      max: this.selectedRange?.end,
    };
    this.plotElement.value!.options = options;
  }

  // Add all the event listeners.
  private addEventListeners(): void {
    // If the user toggles the theme to/from darkmode then redraw.
    document.addEventListener('theme-chooser-toggle', () => {
      // Update the options to trigger the redraw.
      if (this.plotElement.value) {
        this.plotElement.value!.options = mainChartOptions(
          getComputedStyle(this),
          this.domain,
          titleFormatter(getTitle(this.dataframe!))
        );
      }
      this.requestUpdate();
    });

    // We add listeners on the window so we can still track even the mouse is outside the chart
    // area.
    window.addEventListener('mousemove', (e) => {
      this.onWindowMouseMove(e);
    });

    window.addEventListener('mouseup', () => {
      this.onWindowMouseUp();
    });
  }

  private onSidePanelToggle() {
    this.plotElement.value?.redraw();
  }

  private onChartMouseDown(e: MouseEvent) {
    // If the chart emits onmouseover/out events, meaning the mouse is hovering over a data point,
    // we will not try to initiate other chart navigation modes.
    if (this.chartInteracting) {
      return;
    }

    // if user holds down shift-click, enable delta range calculation
    if (e.shiftKey) {
      e.preventDefault(); // disable system events
      this.navigationMode = 'deltaY';
      const layout = this.chart!.getChartLayoutInterface();
      const area = layout.getChartAreaBoundingBox();
      const deltaRangeBox = this.deltaRangeBox.value!;
      deltaRangeBox.show(
        { top: area.top, left: area.left, width: area.width },
        { coord: e.offsetY, value: layout.getVAxisValue(e.offsetY) }
      );
      return;
    }

    // This disable system events like selecting texts.
    e.preventDefault();
    this.navigationMode = 'pan';
    this.lastMouse = { x: e.x, y: e.y };
  }

  private onChartMouseOver() {
    this.chartInteracting = true;
  }

  private onChartMouseUp() {
    this.chartInteracting = false;
    this.navigationMode = null;
    this.deltaRangeBox.value?.hide();
  }

  private onChartMouseOut() {
    this.chartInteracting = false;
    this.navigationMode = null;
    this.deltaRangeBox.value?.hide();
  }

  private onWindowMouseMove(e: MouseEvent) {
    if (this.navigationMode === 'deltaY') {
      e.preventDefault(); // disable system events
      const layout = this.chart!.getChartLayoutInterface();
      const deltaRangeBox = this.deltaRangeBox.value!;
      deltaRangeBox.updateSelection({
        coord: e.offsetY,
        value: layout.getVAxisValue(e.offsetY),
      });
      return;
    }

    if (this.navigationMode !== 'pan') {
      return;
    }

    const deltaX = (this.lastMouse.x - e.x) * this.valueDelta;
    this.lastMouse.x = e.x;

    this.selectedRange!.begin += deltaX;
    this.selectedRange!.end += deltaX;
    this.updateOptions();

    this.dispatchEvent(
      new CustomEvent<PlotSelectionEventDetails>('selection-changing', {
        bubbles: true,
        composed: true,
        detail: {
          value: this.selectedRange!,
          domain: 'commit', // Currently, it is always commit as y-axis.
        },
      })
    );
  }

  private onWindowMouseUp() {
    if (this.navigationMode === 'pan') {
      this.dispatchEvent(
        new CustomEvent<PlotSelectionEventDetails>('selection-changed', {
          bubbles: true,
          composed: true,
          detail: {
            value: this.selectedRange!,
            domain: 'commit', // Currently, it is always commit as y-axis.
          },
        })
      );
    }
    this.navigationMode = null;
    this.chartInteracting = false;
  }

  private drawAnomaly(chart: google.visualization.CoreChartBase) {
    const layout = chart.getChartLayoutInterface();
    if (!this.anomalyMap) {
      return;
    }

    const anomalyDiv = this.anomalyDiv.value;
    if (!anomalyDiv) {
      return;
    }

    const traceset = this.dataframe?.traceset;
    const header = this.dataframe?.header;
    const traceKeys = this.selectedTraces ?? Object.keys(this.anomalyMap);
    const chartRect = layout.getChartAreaBoundingBox();
    const left = chartRect.left,
      top = chartRect.top;
    const right = left + chartRect.width,
      bottom = top + chartRect.height;
    const allDivs: Node[] = [];

    // Clone from the given template icons in the named slots.
    // Each anomaly will clone a new icon element from the template slots and be placed in the
    // anomaly container.
    const slots = this.slots;
    const cloneSlot = (
      name: 'untriage' | 'improvement' | 'regression',
      traceKey: string,
      commit: number
    ) => {
      const assigned = slots[name].value!.assignedElements();
      if (!assigned) {
        console.warn(
          'could not find anomaly template for commit',
          `at ${commit} of ${traceKey} (${name}).`
        );
        return null;
      }
      if (assigned.length > 1) {
        console.warn(
          'multiple children found but only use the first one for commit',
          `at ${commit} of ${traceKey} (${name}).`
        );
      }
      const cloned = assigned[0].cloneNode(true) as HTMLElement;
      cloned.className = `anomaly ${name}`;
      return cloned;
    };

    traceKeys.forEach((key) => {
      const anomalies = this.anomalyMap![key];
      const trace = traceset![key];
      for (const cp in anomalies) {
        const offset = Number(cp);
        const idx = header!.findIndex((v) => v?.offset === offset);
        if (idx < 0) {
          console.warn('anomaly data is out of existing dataframe, ignored.');
          continue;
        }
        const value = trace[idx];
        const x = layout.getXLocation(offset);
        const y = layout.getYLocation(value);
        // We only place the anomaly icons if they are within the chart boundary.
        // p.s. the top and bottom are reversed-coordinated.
        if (x < left || x > right || y < top || y > bottom) {
          continue;
        }

        const anomaly = anomalies[offset];
        let cloned: HTMLElement | null;
        if (anomaly.bug_id <= 0) {
          // no bug assigned, untriaged anomaly
          cloned = cloneSlot('untriage', key, offset);
        } else if (anomaly.is_improvement) {
          cloned = cloneSlot('improvement', key, offset);
        } else {
          cloned = cloneSlot('regression', key, offset);
        }

        if (cloned) {
          cloned.style.top = `${y}px`;
          cloned.style.left = `${x}px`;
          allDivs.push(cloned);
        }
      }
    });
    // replaceChildren API could be already the most efficient API to replace all the children
    // nodes. Alternatively, we could cache all the existing elements for each buckets and place
    // them to the new locations, but the rendering internal may already do this for us.
    // We should only do this optimization if we see a performance issue.
    anomalyDiv.replaceChildren(...allDivs);
  }

  private onChartReady(e: CustomEvent) {
    this.chart = e.detail.chart as google.visualization.CoreChartBase;
    // Only draw the anomaly when the chart is ready.
    this.drawAnomaly(this.chart);

    const layout = this.chart.getChartLayoutInterface();
    const area = layout.getChartAreaBoundingBox();

    if (
      area.left !== this.cachedChartArea.left ||
      area.top !== this.cachedChartArea.top ||
      area.height !== this.cachedChartArea.height ||
      area.width !== this.cachedChartArea.width
    ) {
      this.cachedChartArea = area;
      this.valueDelta = layout.getHAxisValue(area.left + 1) - layout.getHAxisValue(area.left);
    }
  }

  // TODO(b/362831653): deprecate this, no longer needed
  public updateChartData(_chartData: any) {}

  /**
   * Get the (x,y) position in the chart given row and column
   * of the dataframe.
   * @param index An index containing the row and column indexes.
   */
  getPositionByIndex(index: { row: number; col: number }): { x: number; y: number } {
    if (!this.chart) {
      return { x: 0, y: 0 };
    }
    const layout = (this.chart as google.visualization.LineChart).getChartLayoutInterface();
    const rows = this.plotElement.value!.data! as any[][];
    const row = rows[index.row + 1];
    const commitPos = row[0];
    return {
      x: layout.getXLocation(commitPos),
      y: layout.getYLocation(row[index.col]),
    };
  }

  /**
   * Get the commit position of a trace given the row index
   * of the dataframe. The row index represents the x-position
   * of the data. The commit position is always in the first
   * in the first column.
   * @param row The row index
   */
  getCommitPosition(row: number) {
    const rows = this.plotElement.value!.data! as any[][];
    const commitPos = rows[row + 1][0];
    return commitPos;
  }

  /**
   * Get the trace name of a trace given the column index
   * of the dataframe. The trace name is always in the first
   * row. This method makes no modifications to the trace name.
   * @param col The col index
   */
  getTraceName(col: number): string {
    // TODO(b/370804498): Create another getTraceName method that
    // returns a prettified version of the name. i.e.
    // ,arch=x86,config=8888,test=decode,units=kb, becomes
    // x86/8888/decode/kb.
    const rows = this.plotElement.value!.data! as any[][];
    return rows[0][col];
  }

  /**
   * Get the Y value of a trace given the row and column index
   * of the dataframe.
   * @param index An index containing the row and column indexes.
   */
  getYValue(index: { row: number; col: number }): number {
    const rows = this.plotElement.value!.data! as any[][];
    return rows[index.row + 1][index.col];
  }
}

define('plot-google-chart-sk', PlotGoogleChartSk);

declare global {
  interface HTMLElementTagNameMap {
    'plot-google-chart-sk': PlotGoogleChartSk;
  }

  interface GlobalEventHandlersEventMap {
    'selection-changing': CustomEvent<PlotSelectionEventDetails>;
    'selection-changed': CustomEvent<PlotSelectionEventDetails>;
  }
}
