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
import { AnomalyMap } from '../json';
import { mainChartOptions } from '../common/plot-builder';
import {
  dataframeAnomalyContext,
  dataframeLoadingContext,
  dataframeUserIssueContext,
  DataTable,
  dataTableContext,
  UserIssueMap,
} from '../dataframe/dataframe_context';
import { isSingleTrace } from '../dataframe/traceset';
import { range } from '../dataframe/index';
import { VResizableBoxSk } from './v-resizable-box-sk';
import { SidePanelSk } from './side-panel-sk';

export interface PlotSelectionEventDetails {
  value: range;
  domain: 'commit' | 'date';
}

export interface PlotShowTooltipEventDetails {
  row: number;
  col: number;
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
        border: darkblue 1px solid;
      }
      md-icon.improvement {
        background-color: rgba(0, 155, 0, 0.8); /* green */
      }
      md-icon.untriage {
        background-color: rgba(255, 255, 0, 1); /* yellow */
      }
      md-icon.regression {
        background-color: rgba(155, 0, 255, 1); /* purple */
      }
      md-icon.ignored {
        background-color: rgba(100, 100, 100, 0.8); /* grey */
      }
    }

    .userissue {
      position: absolute;
      top: 0px;
      left: 0px;

      md-icon {
        transform: translate(-50%, -50%);
        border-radius: 50%;
        pointer-events: none;
        font-weight: bolder;
        font-size: 12px;
        color: var(--primary);
      }
      md-icon.issue {
        background: var(--background);
        border: white 1px solid;
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

  @consume({ context: dataTableContext, subscribe: true })
  @property({ attribute: false })
  data: DataTable = null;

  @property({})
  selectedTraces: string[] | null = null;

  @property({ reflect: true })
  domain: 'commit' | 'date' = 'commit';

  @property({ attribute: false })
  selectedRange?: range;

  @consume({ context: dataframeAnomalyContext, subscribe: true })
  @property({ attribute: false })
  anomalyMap: AnomalyMap = {};

  @consume({ context: dataframeUserIssueContext, subscribe: true })
  @property({ attribute: false })
  private userIssues: UserIssueMap = {};

  // The slots to place in the templated icons for anomalies.
  private slots = {
    untriage: createRef<HTMLSlotElement>(),
    regression: createRef<HTMLSlotElement>(),
    improvement: createRef<HTMLSlotElement>(),
    ignored: createRef<HTMLSlotElement>(),
    issue: createRef<HTMLSlotElement>(),
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
  // Commits and dates operate on different scales, so adjust accordingly.
  private valueDelta = { commit: 1, date: 1000 };

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

  // The div container for user issue overlays.
  private userIssueDiv = createRef<HTMLDivElement>();

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
        <div class="userissue" ${ref(this.userIssueDiv)}></div>
        <v-resizable-box-sk ${ref(this.deltaRangeBox)}} @mouseup=${this.onChartMouseUp}>
        </v-resizable-box-sk>
        <div class="side" ?hidden=${isSingleTrace(this.data) ?? true}>
          <side-panel-sk ${ref(this.sidePanel)} @side-panel-toggle=${this.onSidePanelToggle}>
          </side-panel-sk>
        </div>
      </div>
      <slot name="untriage" ${ref(this.slots.untriage)}></slot>
      <slot name="regression" ${ref(this.slots.regression)}></slot>
      <slot name="improvement" ${ref(this.slots.improvement)}></slot>
      <slot name="ignored" ${ref(this.slots.ignored)}></slot>
      <slot name="issue" ${ref(this.slots.issue)}></slot>
    `;
  }

  protected willUpdate(changedProperties: PropertyValues): void {
    if (changedProperties.has('anomalyMap')) {
      // If the anomalyMap is getting updated,
      // trigger the chart to redraw and plot the anomaly.
      this.plotElement.value?.redraw();
    } else if (changedProperties.has('userIssues')) {
      this.plotElement.value?.redraw();
    } else if (changedProperties.has('selectedRange')) {
      // If only the selectedRange is updated, then we only update the viewWindow.
      this.updateOptions();
    } else if (changedProperties.has('domain')) {
      this.toggleSelectionRange();
      this.updateDataView(this.data);
    } else if (changedProperties.has('data')) {
      this.updateDataView(this.data);
    }
  }

  private async updateDataView(dt: DataTable) {
    await this.updateComplete;
    const plot = this.plotElement.value;
    if (!plot || !dt) {
      if (dt) {
        console.warn('The datatable is not assigned because the element is not ready.');
      }
      return;
    }

    const view = new google.visualization.DataView(dt!);
    const ncols = view.getNumberOfColumns();

    // The first two columns are the commit position and the date.
    const cols = [this.domain === 'commit' ? 0 : 1];
    for (let index = 2; index < ncols; index++) {
      cols.push(index);
    }

    view.setColumns(cols);
    plot.view = view;
    this.updateOptions();
  }

  // if new domain is commit, convert from date to commit and vice-versa
  // the way this function works is imperfect. The ideal way to do this
  // is for explore-simple-sk to be aware of the selection range from
  // plot-summary-sk and can pass that to plot-google-chart. However,
  // explore-simple-sk is not a lit module and toggling the domain using
  // event listeners could create more complications
  // Instead, we use the existing range and query the data to determine
  // the new range
  // TODO(b/362831653): Fix frame shifting from toggling domain
  // Not sure the cause, could be the timezone?
  private toggleSelectionRange() {
    const newScale = this.domain === 'commit';
    const currBegin = newScale
      ? (new Date(this.selectedRange!.begin! * 1000) as any)
      : this.selectedRange!.begin!;
    const currEnd = newScale
      ? (new Date(this.selectedRange!.end! * 1000) as any)
      : this.selectedRange!.end!;
    const fromCol = newScale ? 1 : 0;
    const toCol = newScale ? 0 : 1;
    const rows = this.data!.getFilteredRows([
      {
        column: fromCol,
        minValue: currBegin,
        maxValue: currEnd,
      },
    ]);
    const begin = this.data!.getValue(rows[0], toCol);
    const end = this.data!.getValue(rows[rows.length - 1], toCol);
    this.selectedRange = {
      begin: newScale ? begin : (begin as any).getTime() / 1000,
      end: newScale ? end : (end as any).getTime() / 1000,
    };
  }

  private updateOptions() {
    const plot = this.plotElement.value;
    if (!plot) {
      return;
    }
    const options = mainChartOptions(getComputedStyle(this), this.domain);
    const begin = this.selectedRange?.begin;
    const end = this.selectedRange?.end;
    const commitScale = this.domain === 'commit';

    options.hAxis!.viewWindow = {
      min: commitScale ? begin : (new Date(begin! * 1000) as any),
      max: commitScale ? end : (new Date(end! * 1000) as any),
    };

    plot.options = options;
  }

  // Add all the event listeners.
  private addEventListeners(): void {
    // If the user toggles the theme to/from darkmode then redraw.
    document.addEventListener('theme-chooser-toggle', () => {
      // Update the options to trigger the redraw.
      if (this.plotElement.value) {
        this.plotElement.value!.options = mainChartOptions(getComputedStyle(this), this.domain);
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

  private onChartMouseOver(e: CustomEvent) {
    if (this.navigationMode === 'deltaY') {
      return;
    }
    this.chartInteracting = true;
    this.dispatchEvent(
      new CustomEvent<PlotShowTooltipEventDetails>('plot-data-mouseover', {
        bubbles: true,
        composed: true,
        detail: {
          row: e.detail.data.row,
          col: e.detail.data.column,
        },
      })
    );
  }

  private onChartMouseUp() {
    this.chartInteracting = false;
    this.navigationMode = null;
    this.deltaRangeBox.value?.hide();
  }

  private onChartMouseOut() {
    this.chartInteracting = false;
    this.navigationMode = this.navigationMode === 'deltaY' ? 'deltaY' : null;
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

    const valueDelta = this.domain === 'commit' ? this.valueDelta.commit : this.valueDelta.date;
    const deltaX = (this.lastMouse.x - e.x) * valueDelta;
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
          domain: this.domain,
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
            domain: this.domain,
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

    const data = this.data!;

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
      name: 'untriage' | 'improvement' | 'regression' | 'ignored',
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
      const traceCol = this.data!.getColumnIndex(key)!;
      for (const cp in anomalies) {
        const offset = Number(cp);
        const rows = data.getFilteredRows([{ column: 0, value: offset }]);
        if (rows.length < 0) {
          console.warn('anomaly data is out of existing dataframe, ignored.');
          continue;
        }
        const xValue =
          this.domain === 'commit' ? data.getValue(rows[0], 0) : data.getValue(rows[0], 1);
        const yValue = data.getValue(rows[0], traceCol);
        const x = layout.getXLocation(xValue);
        const y = layout.getYLocation(yValue);
        // We only place the anomaly icons if they are within the chart boundary.
        // p.s. the top and bottom are reversed-coordinated.
        if (x < left || x > right || y < top || y > bottom) {
          continue;
        }

        const anomaly = anomalies[offset];
        let cloned: HTMLElement | null;
        // TODO(b/377751454): add separate anomaly icon for "ignored"
        if (anomaly.is_improvement) {
          cloned = cloneSlot('improvement', key, offset);
        } else if (anomaly.bug_id === 0) {
          // no bug assigned, untriaged anomaly
          cloned = cloneSlot('untriage', key, offset);
        } else if (anomaly.bug_id < 0) {
          cloned = cloneSlot('ignored', key, offset);
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

  private drawUserIssues(chart: google.visualization.CoreChartBase) {
    const layout = chart.getChartLayoutInterface();
    if (!this.userIssues) {
      return;
    }

    const userIssueDiv = this.userIssueDiv.value;
    if (!userIssueDiv) {
      return;
    }

    const data = this.data!;

    const traceKeys = this.selectedTraces ?? Object.keys(this.userIssues);
    const chartRect = layout.getChartAreaBoundingBox();
    const left = chartRect.left,
      top = chartRect.top;
    const right = left + chartRect.width,
      bottom = top + chartRect.height;
    const allDivs: Node[] = [];

    // Clone from the given template icons in the named slots.
    const slots = this.slots;
    const cloneSlot = (name: 'issue', traceKey: string, commit: number) => {
      const assigned = slots[name].value!.assignedElements();
      if (!assigned || assigned.length === 0) {
        console.warn(
          'could not find user issue template for commit',
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
      cloned.className = `userissue ${name}`;
      return cloned;
    };

    traceKeys.forEach((key) => {
      const userIssues = this.userIssues![key];
      const traceCol = this.data!.getColumnIndex(key)!;
      for (const [cp, issueDetail] of Object.entries(userIssues)) {
        const offset = Number(cp);
        const anomaliesOnTraces = this.anomalyMap![key];
        if (anomaliesOnTraces !== null && anomaliesOnTraces !== undefined) {
          const a = anomaliesOnTraces[offset];
          if (a !== null && a !== undefined) {
            console.warn('user issue same as anomaly, ignored.');
            continue;
          }
        }

        const rows = data.getFilteredRows([{ column: 0, value: offset }]);
        if (rows.length < 0) {
          console.warn('user issue data is out of existing dataframe, ignored.');
          continue;
        }
        if (issueDetail.bugId === 0) {
          continue;
        }
        const xValue =
          this.domain === 'commit' ? data.getValue(rows[0], 0) : data.getValue(rows[0], 1);
        const yValue = data.getValue(rows[0], traceCol);
        const x = layout.getXLocation(xValue);
        const y = layout.getYLocation(yValue);
        // We only place the user issue icons if they are within the chart boundary.
        // p.s. the top and bottom are reversed-coordinated.
        if (x < left || x > right || y < top || y > bottom) {
          continue;
        }

        const cloned: HTMLElement | null = cloneSlot('issue', key, offset);

        if (cloned) {
          cloned.style.top = `${y}px`;
          cloned.style.left = `${x}px`;
          allDivs.push(cloned);
        }
      }
    });

    userIssueDiv.replaceChildren(...allDivs);
  }

  private onChartReady(e: CustomEvent) {
    this.chart = e.detail.chart as google.visualization.CoreChartBase;
    // Only draw the anomaly when the chart is ready.
    this.drawAnomaly(this.chart);
    this.drawUserIssues(this.chart);

    const layout = this.chart.getChartLayoutInterface();
    const area = layout.getChartAreaBoundingBox();

    if (
      area.left !== this.cachedChartArea.left ||
      area.top !== this.cachedChartArea.top ||
      area.height !== this.cachedChartArea.height ||
      area.width !== this.cachedChartArea.width
    ) {
      this.cachedChartArea = area;
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
    const domainColumn = this.domain === 'commit' ? 0 : 1;
    const layout = (this.chart as google.visualization.LineChart).getChartLayoutInterface();
    const xValue = this.data!.getValue(index.row, domainColumn);
    const yValue = this.data!.getValue(index.row, index.col + 1);
    return {
      x: layout.getXLocation(xValue),
      y: layout.getYLocation(yValue),
    };
  }

  /**
   * Get the commit position of a trace given the row index
   * of the DataTable. The row index represents the x-position
   * of the data. The commit position is always in the first
   * in the first column.
   * @param row The row index
   */
  getCommitPosition(row: number) {
    return this.data!.getValue(row + 1, 0);
  }

  /**
   * Get the trace name of a trace given the column index
   * of the DataTable. The trace name is always in the first
   * row. This method makes no modifications to the trace name.
   * @param col The col index
   */
  getTraceName(col: number): string {
    // TODO(b/370804498): Create another getTraceName method that
    // returns a prettified version of the name. i.e.
    // ,arch=x86,config=8888,test=decode,units=kb, becomes
    // x86/8888/decode/kb.
    // first two columns of DataTable are commit position and date
    return this.data!.getColumnLabel(col + 1);
  }

  /**
   * Get the Y value of a trace given the row and column index
   * of the dataframe.
   * @param index An index containing the row and column indexes.
   */
  getYValue(index: { row: number; col: number }): number {
    // first two columns of DataTable are commit position and date
    return this.data!.getValue(index.row, index.col + 1);
  }

  /**
   * Unselect all selections on the chart.
   */
  unselectAll(): void {
    this.chart!.setSelection([]);
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
    'plot-data-mouseover': CustomEvent<PlotShowTooltipEventDetails>;
    'plot-chart-mousedown': CustomEvent;
  }
}
