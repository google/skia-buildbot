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
import { defaultColors, mainChartOptions } from '../common/plot-builder';
import {
  dataframeAnomalyContext,
  dataframeLoadingContext,
  dataframeUserIssueContext,
  DataTable,
  dataTableContext,
  UserIssueMap,
} from '../dataframe/dataframe_context';
import { findTraceByLabel, findTracesForParam, isSingleTrace } from '../dataframe/traceset';
import { range } from '../dataframe/index';
import { VResizableBoxSk } from './v-resizable-box-sk';
import { SidePanelCheckboxClickDetails, SidePanelSk } from './side-panel-sk';
import { DragToZoomBox } from './drag-to-zoom-box-sk';

export interface PlotSelectionEventDetails {
  value: range;
  domain: 'commit' | 'date';
}

export interface PlotShowTooltipEventDetails {
  tableRow: number;
  tableCol: number;
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
        background-color: rgba(255, 0, 0, 1); /* red */
      }
      md-icon.ignored {
        background-color: rgba(100, 100, 100, 0.8); /* grey */
      }
      md-icon.highlighted {
        outline: 3px solid var(--warning);
        outline-offset: 2px;
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

    .xbar {
      position: absolute;
      top: 0px;
      left: 0px;

      md-text {
        pointer-events: none;
        font-weight: bolder;
        transform: translate(-50%, -50%);
        font-size: 60px;
        color: var(--error);
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

    .closeIcon {
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

  @property({ attribute: false })
  private deltaRangeOn = false;

  @property({ attribute: false })
  private showResetButton = false;

  @property({ type: Array })
  highlightAnomalies: string[] = [];

  // The slots to place in the templated icons for anomalies.
  private slots = {
    untriage: createRef<HTMLSlotElement>(),
    regression: createRef<HTMLSlotElement>(),
    improvement: createRef<HTMLSlotElement>(),
    ignored: createRef<HTMLSlotElement>(),
    issue: createRef<HTMLSlotElement>(),
    xbar: createRef<HTMLSlotElement>(),
  };

  // Modes for chart interaction with mouse.
  // We have panning, deltaY and dragToZoom for now.
  // Default behavior is null.
  // - panning (enabled by left click dragging) pans the chart to the left or right
  // - deltaY (enabled with shift-click) calculates the delta on the
  // y-axis between the start and end cursor.
  // - dragToZoom enable to vertical and horizontal zoom, by ctrl click to drag the area
  @property({ attribute: false })
  private navigationMode: 'pan' | 'deltaY' | 'dragToZoom' | null = null;

  // Vertical zoom by default
  @property({ attribute: false })
  isHorizontalZoom = false;

  // The location of the XBar. See the xbar property.
  @property({ attribute: false })
  xbar: number = -1;

  private lastMouse = { x: 0, y: 0 };

  // Maps a trace to a color.
  private traceColorMap = new Map<string, string>();

  // Index to keep track of which colors we've used so far.
  private colorIndex = 0;

  private cachedChartArea = { left: 0, top: 0, width: 0, height: 0 };

  // Whether we are interacting with the chart that takes higher prioritiy than navigations.
  private chartInteracting = false;

  // cache the googleChart object within the module
  private chart: google.visualization.CoreChartBase | null = null;

  // cache the labels which were removed, so that they can be easily re-added
  private removedLabelsCache: string[] = [];

  // The div element that will host the plot on the summary.
  private plotElement: Ref<GoogleChart> = createRef();

  // The div container for anomaly overlays.
  private anomalyDiv = createRef<HTMLDivElement>();

  // The div container for user issue overlays.
  private userIssueDiv = createRef<HTMLDivElement>();

  // The div container for delta y selection range.
  private deltaRangeBox = createRef<VResizableBoxSk>();

  // The div container for zoom selection range.
  private zoomRangeBox = createRef<DragToZoomBox>();

  // The div container for the legend
  private sidePanel = createRef<SidePanelSk>();

  // The div container for anomaly overlays.
  private xbarDiv = createRef<HTMLDivElement>();

  constructor() {
    super();
    this.addEventListeners();
  }

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
          @google-chart-select=${this.onChartSelect}
          @google-chart-onmouseover=${this.onChartMouseOver}
          @google-chart-onmouseout=${this.onChartMouseOut}
          @google-chart-ready=${this.onChartReady}>
        </google-chart>
        ${when(this.loading, () => html`<md-linear-progress indeterminate></md-linear-progress>`)}
        <div class="anomaly" ${ref(this.anomalyDiv)}></div>
        <div class="userissue" ${ref(this.userIssueDiv)}></div>
        <div class="xbar" ${ref(this.xbarDiv)}></div>
        <v-resizable-box-sk ${ref(this.deltaRangeBox)}} @mouseup=${this.onChartMouseUp}>
        </v-resizable-box-sk>
        <drag-to-zoom-box-sk ${ref(this.zoomRangeBox)}} @mouseup=${this.onChartMouseUp}>
        </drag-to-zoom-box-sk>
        <div id="reset-view" ?hidden=${!this.showResetButton}>
          <button id="closeIcon" @click=${this.resetView}>Reset to original view</button>
        </div>
        <div class="side" ?hidden=${isSingleTrace(this.data) ?? true}>
          <side-panel-sk
            ${ref(this.sidePanel)}
            @side-panel-toggle=${this.onSidePanelToggle}
            @side-panel-selected-trace-change=${this.sidePanelCheckboxUpdate}>
          </side-panel-sk>
        </div>
      </div>
      <slot name="untriage" ${ref(this.slots.untriage)}></slot>
      <slot name="regression" ${ref(this.slots.regression)}></slot>
      <slot name="improvement" ${ref(this.slots.improvement)}></slot>
      <slot name="ignored" ${ref(this.slots.ignored)}></slot>
      <slot name="issue" ${ref(this.slots.issue)}></slot>
      <slot name="xbar" ${ref(this.slots.xbar)}></slot>
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
    const hiddenColumns = [];
    for (let index = 2; index < ncols; index++) {
      const label = view.getColumnLabel(index);
      cols.push(index);

      // Assign a specific color to all labels.
      if (!this.traceColorMap.has(label)) {
        this.traceColorMap.set(label, defaultColors[this.colorIndex % defaultColors.length]);
        this.colorIndex++;
      }

      if (this.removedLabelsCache.includes(view.getColumnLabel(index))) {
        hiddenColumns.push(index);
      }
    }
    view.setColumns(cols);

    if (this.removedLabelsCache.length > 0) {
      view.hideColumns(hiddenColumns);
    }

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
    const options = mainChartOptions(
      getComputedStyle(this),
      this.domain,
      this.determineYAxisTitle(this.getAllTraces())
    );
    const begin = this.selectedRange?.begin;
    const end = this.selectedRange?.end;
    const commitScale = this.domain === 'commit';
    options.hAxis!.viewWindow = {
      min: commitScale ? begin : (new Date(begin! * 1000) as any),
      max: commitScale ? end : (new Date(end! * 1000) as any),
    };

    options.colors = [];
    // Get internal indices of visible columns.
    const visibleColumns = plot.view!.getViewColumns();
    for (const colIndex of visibleColumns) {
      // skip first two indices as these are reserved.
      if (colIndex > 1) {
        // Translate those internal indices to indices of visible columns.
        const tableIndex = plot.view!.getViewColumnIndex(colIndex);
        const label = plot.view!.getColumnLabel(tableIndex);
        options.colors.push(this.traceColorMap.get(label)!);
      }
    }
    plot.options = options;
  }

  /**
   * determineYAxisTitle determines the Y axis title based on the traceNames.
   *
   * There are two properties that we aim to display on the Y axis: unit, and
   * improvement direction.
   *
   * 1. All traces have the same unit, and same improvement direction
   * 2. Same unit, different improvement direction
   * 3. Different unit, same improvement direction
   * 4. Different unit, different improvement direction
   *
   * This function will only display fields that align. For this function to
   * display unit and improvement direction, they must be set as part of the
   * trace name.
   *
   * There are a few assumptions made as part of this function. For starters,
   * the keys "unit" and "improvement_direction" (literal, case sensitive) must
   * be a part of the trace name for this function to display them.
   * Additionally, this function requires a comma delimited k/v pairs in the
   * format of k=v. For example, in Chromium, ",unit=score,".
   */
  determineYAxisTitle(traceNames: string[]): string {
    if (traceNames.length < 1) {
      return '';
    }

    // traceParams is a list of k/v pairs, in format {k}={v}. returns val.
    function parseVal(key: string, traceParams: string[]): string {
      for (const kv of traceParams) {
        if (kv.startsWith(key)) {
          const pieces = kv.split('=', 2);
          return pieces[1];
        }
      }

      return '';
    }

    let idx = 0;
    let params = traceNames[idx].split(',');
    let unit = parseVal('unit', params);
    let improvement_dir = parseVal('improvement_dir', params);

    for (idx = 1; idx < traceNames.length; idx++) {
      params = traceNames[idx].split(',');
      // unset if values are not the same across all traces
      if (unit !== parseVal('unit', params)) {
        unit = '';
      }
      if (improvement_dir !== parseVal('improvement_dir', params)) {
        improvement_dir = '';
      }

      // early termination
      if (unit === '' && improvement_dir === '') {
        return '';
      }
    }

    let title = '';
    if (unit !== '') {
      title += `${unit}`;
    }
    if (improvement_dir !== '') {
      if (unit !== '') {
        // if unit is not the same and improvement direction is, only display
        // we don't want to append this hyphen
        title += ' - ';
      }
      title += `${improvement_dir}`;
    }
    return title;
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
          this.determineYAxisTitle(this.getAllTraces())
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

    // Event listener for when the "Switch zoom direction" button is selected.
    // It will switch the zoomin feature between horizontal and vertical
    document.addEventListener('switch-zoom', (e) => {
      this.isHorizontalZoom = (e as CustomEvent).detail.key;
      this.requestUpdate();
    });
  }

  private onSidePanelToggle() {
    this.plotElement.value?.redraw();
  }

  /**
   * Handler for the event when the side panel checkbox is clicked.
   * @param e Checkbox click event with details containing the selected
   * state of the checkbox and the metric subtest values.
   */
  private sidePanelCheckboxUpdate(e: CustomEvent<SidePanelCheckboxClickDetails>) {
    const isCheckedboxSelected = e.detail.selected;
    const labelsList = e.detail.labels;
    labelsList.forEach((label) => {
      const trace = findTraceByLabel(this.data, label);
      if (trace === null) {
        console.warn('Could not find trace for ', label);
        return;
      }
      if (isCheckedboxSelected) {
        this.removedLabelsCache = this.removedLabelsCache.filter((label) => label !== trace);
      } else {
        this.removedLabelsCache.push(trace);
      }
    });
    this.updateDataView(this.data);
  }

  private onChartMouseDown(e: MouseEvent) {
    const layout = this.chart!.getChartLayoutInterface();
    const area = layout.getChartAreaBoundingBox();
    // if user holds down shift-click, enable delta range calculation
    if (e.shiftKey) {
      e.preventDefault(); // disable system events
      this.deltaRangeOn = !this.deltaRangeOn;
      this.navigationMode = 'deltaY';
      const deltaRangeBox = this.deltaRangeBox.value!;
      deltaRangeBox.show(
        { top: area.top, left: area.left, width: area.width },
        { coord: e.offsetY, value: layout.getVAxisValue(e.offsetY) }
      );
      return;
    } else if (this.navigationMode === 'deltaY') {
      this.deltaRangeOn = !this.deltaRangeOn;
    } else if (e.ctrlKey) {
      e.preventDefault(); // disable system events
      this.navigationMode = 'dragToZoom';
      const zoomRangeBox = this.zoomRangeBox.value!;

      zoomRangeBox.initializeShow(
        { top: area.top, left: area.left, width: area.width, height: area.height },
        { xOffset: e.offsetX, yOffset: e.offsetY }
      );
      this.lastMouse = { x: e.offsetX, y: e.offsetY };
      return;
    }
    // This disable system events like selecting texts.
    e.preventDefault();
    this.deltaRangeBox.value?.hide();
    this.zoomRangeBox.value?.hide();
    this.navigationMode = 'pan';
    this.lastMouse = { x: e.x, y: e.y };
    this.dispatchEvent(
      new CustomEvent('plot-chart-mousedown', {
        bubbles: true,
        composed: true,
      })
    );
  }

  // When a point is hovered, return row and column values from
  // underlying data table.
  private onChartMouseOver(e: CustomEvent) {
    if (this.navigationMode === 'deltaY' || this.navigationMode === 'dragToZoom') {
      return;
    }
    this.chartInteracting = true;
    // The detail will contain the row and column values for the View, that
    // is the indices of the visible traces. If some traces are hidden, we need
    // to translate the visible indices to the actual table indices.
    // These are the indices that should be used when using the methods in this.data.
    const plot = this.plotElement.value;
    const tableRowIndex = plot!.view!.getTableRowIndex(e.detail.data.row);
    const tableColumnIndex = plot!.view!.getTableColumnIndex(e.detail.data.column);

    this.dispatchEvent(
      new CustomEvent<PlotShowTooltipEventDetails>('plot-data-mouseover', {
        bubbles: true,
        composed: true,
        detail: {
          tableRow: tableRowIndex,
          tableCol: tableColumnIndex,
        },
      })
    );
  }

  // When a point is selected, return row and column values from
  // underlying data table.
  private onChartSelect(e: CustomEvent) {
    if (this.navigationMode === 'deltaY' || this.navigationMode === 'dragToZoom') {
      return;
    }
    this.chartInteracting = true;

    const selection = e.detail.chart.getSelection()[0];
    if (selection === undefined) {
      return;
    }

    const plot = this.plotElement.value;
    const tableRowIndex = plot!.view!.getTableRowIndex(selection.row);
    const tableColumnIndex = plot!.view!.getTableColumnIndex(selection.column);

    this.dispatchEvent(
      new CustomEvent<PlotShowTooltipEventDetails>('plot-data-select', {
        bubbles: true,
        composed: true,
        detail: {
          tableRow: tableRowIndex,
          tableCol: tableColumnIndex,
        },
      })
    );
  }

  private onChartMouseUp(e: MouseEvent) {
    const layout = this.chart!.getChartLayoutInterface();
    this.sidePanel.value!.showDelta = this.deltaRangeOn;
    if (this.navigationMode === 'deltaY') {
      if (Number(this.deltaRangeBox.value!.getDelta())) {
        this.sidePanel.value!.deltaRaw = Number(this.deltaRangeBox.value!.getDelta()!.raw!);
        this.sidePanel.value!.deltaPercentage = Number(
          this.deltaRangeBox.value!.getDelta()!.percent
        );
      } else {
        console.warn('delta range is not valid, ignored.');
        return;
      }
    }
    if (this.navigationMode === 'dragToZoom') {
      let zoominRange = { begin: 0, end: 0 };
      // calculates the offset of a mouse click relative to the left edge of a specific element
      let calculatedOffset = 0;
      if (this.isHorizontalZoom) {
        calculatedOffset = e.clientX - this.plotElement.value!.getBoundingClientRect().left;
        // floor the x-axis since we cannot have fractional commits
        zoominRange = {
          begin: Math.floor(layout.getHAxisValue(this.lastMouse.x)),
          end: Math.floor(layout.getHAxisValue(calculatedOffset)),
        };
      } else {
        calculatedOffset = e.clientY - this.plotElement.value!.getBoundingClientRect().top;
        zoominRange = {
          begin: layout.getVAxisValue(this.lastMouse.y),
          end: layout.getVAxisValue(calculatedOffset),
        };
      }
      this.zoomRangeBox.value?.hide();
      this.showResetButton = true;
      this.updateBounds(zoominRange);
    }
    this.deltaRangeOn = false;
    this.chartInteracting = false;
    this.navigationMode = null;
  }

  // this interaction triggers when mousing off of a data point
  // this event listener is to turn off the tooltip after hovering away
  // from a data point
  // this interaction can mess with continuous mousing journeys
  // like deltaRange and zoom so need to ensure they will continue
  // to work even past data points
  private onChartMouseOut() {
    this.chartInteracting = this.navigationMode !== null;
    this.dispatchEvent(
      new CustomEvent('plot-chart-mouseout', {
        bubbles: true,
        composed: true,
      })
    );
  }

  private onWindowMouseMove(e: MouseEvent) {
    // prevents errors while chart is loading up
    if (this.chart === null) {
      return;
    }
    const layout = this.chart.getChartLayoutInterface();
    if (this.navigationMode === 'deltaY') {
      e.preventDefault(); // disable system events
      const deltaRangeBox = this.deltaRangeBox.value!;
      deltaRangeBox.updateSelection({
        coord: e.offsetY,
        value: layout.getVAxisValue(e.offsetY),
      });

      this.sidePanel.value!.showDelta = false;
      this.sidePanel.value!.deltaRaw = Number(this.deltaRangeBox.value!.getDelta()!.raw!);
      this.sidePanel.value!.deltaPercentage = Number(this.deltaRangeBox.value!.getDelta()!.percent);
      return;
    }

    if (this.navigationMode === 'dragToZoom') {
      e.preventDefault(); // disable system events
      const zoomRangeBox = this.zoomRangeBox.value!;
      zoomRangeBox.handleDrag({
        offset: this.isHorizontalZoom ? e.offsetX : e.offsetY,
        isHorizontal: this.isHorizontalZoom,
      });
      return;
    }

    if (this.navigationMode === 'pan') {
      let deltaX = layout.getHAxisValue(this.lastMouse.x) - layout.getHAxisValue(e.x);
      // if date, scale by 1000 to adjust for timescale
      deltaX = this.domain === 'commit' ? deltaX : deltaX / 1000;
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
  }

  private onWindowMouseUp() {
    if (this.navigationMode === 'dragToZoom') {
      this.showResetButton = true;
      this.zoomRangeBox.value?.hide();
      return;
    }
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
      commit: number,
      highlight: boolean
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
      if (highlight) {
        cloned.className = `${cloned.className} highlighted`;
      }
      return cloned;
    };

    traceKeys.forEach((key) => {
      if (!this.removedLabelsCache.includes(key)) {
        const anomalies = this.anomalyMap![key];
        const traceCol = this.data!.getColumnIndex(key)!;
        for (const cp in anomalies) {
          const offset = Number(cp);
          const rows = data.getFilteredRows([{ column: 0, value: offset }]);
          if (rows.length === 0) {
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
          const highlight =
            this.highlightAnomalies && this.highlightAnomalies.includes(anomaly.id.toString());
          if (anomaly.is_improvement) {
            cloned = cloneSlot('improvement', key, offset, highlight);
          } else if (anomaly.bug_id === 0) {
            // no bug assigned, untriaged anomaly
            cloned = cloneSlot('untriage', key, offset, highlight);
          } else if (anomaly.bug_id < 0) {
            cloned = cloneSlot('ignored', key, offset, highlight);
          } else {
            cloned = cloneSlot('regression', key, offset, highlight);
          }

          if (cloned) {
            cloned.style.top = `${y}px`;
            cloned.style.left = `${x}px`;
            allDivs.push(cloned);
          }
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

  private drawXbar(chart: google.visualization.CoreChartBase) {
    const layout = chart.getChartLayoutInterface();
    if (this.xbar === -1) {
      return;
    }

    const xbarDiv = this.xbarDiv.value;
    if (!xbarDiv) {
      return;
    }

    const chartRect = layout.getChartAreaBoundingBox();
    const left = chartRect.left,
      top = chartRect.top;
    const right = left + chartRect.width,
      bottom = top + chartRect.height;
    const allDivs: Node[] = [];

    // Clone from the given template icons in the named slots.
    const slots = this.slots;
    const cloneSlot = (name: 'xbar') => {
      const assigned = slots[name].value!.assignedElements();
      if (!assigned || assigned.length === 0) {
        console.warn('could not find xbar template at ${commit} of ${row}');
        return null;
      }
      if (assigned.length > 1) {
        console.warn('multiple clones found at ${commit} of ${row}');
      }
      const cloned = assigned[0].cloneNode(true) as HTMLElement;
      cloned.className = `${name}`;
      return cloned;
    };

    // Every 10px mark a new line.
    for (let y = top; y < bottom; y += 10) {
      const x = layout.getXLocation(this.xbar);
      // Ensure line can be drawn on visible canvas.
      if (x >= left && x <= right && y >= top && y <= bottom) {
        const cloned: HTMLElement | null = cloneSlot('xbar');

        if (cloned) {
          cloned.style.top = `${y}px`;
          cloned.style.left = `${x}px`;
          allDivs.push(cloned);
        }
      }
    }
    xbarDiv.replaceChildren(...allDivs);
  }

  private onChartReady(e: CustomEvent) {
    this.chart = e.detail.chart as google.visualization.CoreChartBase;
    // Only draw the anomaly when the chart is ready.
    this.drawAnomaly(this.chart);
    this.drawUserIssues(this.chart);
    this.drawXbar(this.chart);

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

  /**
   * When the zoomin drag ends, update bounds in the plot-google-chart by
   * calculating the x and y coordinates of the chart's next frame.
   * begin and end refer to the x-axis or y-axis values of where the cursor
   * started and stopped. They can be in any order.
   */
  updateBounds(zoominRange: { begin: number; end: number }) {
    const zoomRangeBox = this.zoomRangeBox.value!;
    if (zoomRangeBox?.startPosition) {
      const options = mainChartOptions(
        getComputedStyle(this),
        this.domain,
        this.determineYAxisTitle(this.getAllTraces())
      );
      const newScale = this.domain === 'commit';
      const plot = this.plotElement.value;
      const min = Math.min(zoominRange!.begin, zoominRange!.end);
      const max = Math.max(zoominRange!.begin, zoominRange!.end);
      if (this.isHorizontalZoom) {
        options.hAxis!.viewWindow = {
          min: newScale ? min : (new Date(min!) as any),
          max: newScale ? max : (new Date(max!) as any),
        };
      } else if (!this.isHorizontalZoom) {
        options.vAxis!.viewWindow = {
          min: min,
          max: max,
        };
        options.hAxis!.viewWindow = {
          min: this.selectedRange?.begin,
          max: this.selectedRange?.end,
        };
      }
      if (plot) {
        plot.options = options;
      }
    }
  }

  // Reset to original view
  private resetView() {
    const plot = this.plotElement.value;
    const options = mainChartOptions(
      getComputedStyle(this),
      this.domain,
      this.determineYAxisTitle(this.getAllTraces())
    );
    options.hAxis!.viewWindow = {
      min: this.selectedRange?.begin,
      max: this.selectedRange?.end,
    };
    if (plot) {
      plot!.options = options;
      plot!.redraw();
    }
    this.showResetButton = false;
  }

  // TODO(b/362831653): deprecate this, no longer needed
  public updateChartData(_chartData: any) {}

  /**
   * Updates the chart based on the param value provided in the arguments.
   * @param key param key
   * @param val param value
   * @param selected True if the param is selected, else False.
   */
  public updateChartForParam(key: string, val: string, selected: boolean) {
    const tracesForParams = findTracesForParam(this.data, key, val);
    if (selected) {
      // We want only these traces to be visible, so add all others into the removed cache.
      this.removedLabelsCache = this.removedLabelsCache.filter(
        (trace) => !tracesForParams!.includes(trace)
      );
    } else {
      // Params were unselected, so add all the matching traces to the removed cache.
      this.removedLabelsCache = this.removedLabelsCache.concat(tracesForParams!);
    }

    // Update the side panel checkboxes to reflect the state.
    this.updateSidePanel();
  }

  /**
   * Update the side panel checkboxes based on the removed labels cache.
   * This in turn automatically updates the graph data.
   */
  private updateSidePanel() {
    // Take a copy of the labels to remove since the setallboxes can cause the
    // removedlabelscache to update itself.
    const removedLabels = this.removedLabelsCache;
    this.sidePanel.value?.SetAllBoxes(true);
    removedLabels.forEach((label) => {
      this.sidePanel.value?.SetCheckboxForTrace(false, label);
    });
  }

  /**
   * Get the (x,y) position in the chart given row and column
   * of the dataframe.
   * @param index An index containing the row and column indexes.
   */
  getPositionByIndex(index: { tableRow: number; tableCol: number }): { x: number; y: number } {
    if (!this.chart) {
      return { x: 0, y: 0 };
    }
    const domainColumn = this.domain === 'commit' ? 0 : 1;
    const layout = (this.chart as google.visualization.LineChart).getChartLayoutInterface();
    const xValue = this.data!.getValue(index.tableRow, domainColumn);
    const yValue = this.data!.getValue(index.tableRow, index.tableCol);
    return {
      x: layout.getXLocation(xValue),
      y: layout.getYLocation(yValue),
    };
  }

  /**
   * Get the commit position of a trace given the row index
   * of the DataTable. The row index represents the x-position
   * of the data. The commit position is always in the first column.
   * @param row The row index
   */
  getCommitPosition(row: number) {
    return this.data!.getValue(row, 0);
  }

  /**
   * Get the commit date of a trace given the row index
   * of the DataTable. The row index represents the x-position
   * of the data. The commit date is always in the second column.
   * @param row The row index
   */
  getCommitDate(row: number) {
    return this.data!.getValue(row, 1);
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
    return this.data!.getColumnLabel(col);
  }

  /**
   *
   * @returns All traces in string format
   */
  getAllTraces(): string[] {
    const allCols: string[] = [];
    // first two columns are always reserved for 'Commit Position' and 'Date'
    for (let idx = 2; idx < this.data!.getNumberOfColumns(); idx++) {
      allCols.push(this.data!.getColumnLabel(idx));
    }
    return allCols;
  }

  /**
   * Get the Y value of a trace given the row and column index
   * of the dataframe.
   * @param index An index containing the row and column indexes.
   */
  getYValue(index: { tableRow: number; tableCol: number }): number {
    // first two columns of DataTable are commit position and date
    return this.data!.getValue(index.tableRow, index.tableCol);
  }

  /**
   * Unselect all selections on the chart.
   */
  unselectAll(): void {
    if (this.chart === null) {
      return;
    }
    this.chart.setSelection([]);
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
    'plot-data-select': CustomEvent<PlotShowTooltipEventDetails>;
    'plot-chart-mousedown': CustomEvent;
    'plot-chart-mouseout': CustomEvent;
  }
}
