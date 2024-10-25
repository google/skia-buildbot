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
import { GoogleChart } from '@google-web-components/google-chart';

import { consume } from '@lit/context';
import { html, css } from 'lit';
import { LitElement, PropertyValues } from 'lit';
import { ref, Ref, createRef } from 'lit/directives/ref.js';
import { property } from 'lit/decorators.js';
import { define } from '../../../elements-sk/modules/define';
import { Anomaly, AnomalyMap, DataFrame } from '../json';
import { convertFromDataframe, mainChartOptions } from '../common/plot-builder';
import { dataframeAnomalyContext, dataframeContext } from '../dataframe/dataframe_context';
import { getTitle, titleFormatter } from '../dataframe/traceset';
import { DataTableLike } from '@google-web-components/google-chart/loader';
import { range } from '../dataframe/index';

export interface AnomalyData {
  x: number;
  y: number;
  anomaly: Anomaly;
  highlight: boolean;
}

interface Selection {
  row: string;
  column: string;
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
      display: grid;
      grid-template-columns: 4fr 1fr;
      height: 100%;
      width: 100%;
    }
    .plot {
      height: 100%;
      width: 100%;
    }
    .plot-panel {
      font-family: 'Roboto', system-ui, sans-serif;
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

    ul.table {
      list-style: none;
      padding: 0;
      margin: 0;
      width: 100%;
      border-collapse: collapse;
      margin-bottom: 16px;

      li {
        display: table-row;

        span {
          &:first-child {
            font-weight: bold;
          }

          display: table-cell;
          padding: 1px 6px;
        }
      }
    }
  `;

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

  @property()
  private tooltip?: {
    traceName: string;
    value: number;
    commit: string;
    date: string;
  };

  // The slots to place in the templated icons for anomalies.
  private slots = {
    untriage: createRef<HTMLSlotElement>(),
    regression: createRef<HTMLSlotElement>(),
    improvement: createRef<HTMLSlotElement>(),
  };

  // How to pan or zoom the chart, we only have panning now.
  private navigationMode: 'pan' | null = null;

  private lastMouse = { x: 0, y: 0 };

  // The value distance when moving by 1px on the screen.
  private valueDelta = 1;

  private cachedChartArea = { left: 0, top: 0, width: 0, height: 0 };

  // Whether we are interacting with the chart that takes higher prioritiy than navigations.
  private chartInteracting = false;

  constructor() {
    super();

    this.addEventListeners();
  }

  // The div element that will host the plot on the summary.
  private plotElement: Ref<GoogleChart> = createRef();

  // The div container for anomaly overlays.
  private anomalyDiv = createRef<HTMLDivElement>();

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
          @google-chart-onmouseover=${this.onChartMouseOver}
          @google-chart-onmouseout=${this.onChartMouseOut}
          @google-chart-ready=${this.onChartReady}
          @google-chart-select=${this.onSelectDataPoint}>
        </google-chart>
        <div class="anomaly" ${ref(this.anomalyDiv)}></div>
        <div class="plot-panel" ?hidden=${!this.tooltip}>
          <h3>${this.tooltip?.traceName}</h3>
          <ul class="table">
            <li>
              <span>Value:</span>
              <span>${this.tooltip?.value}</span>
            </li>
          </ul>
          <ul class="table">
            <li>
              <span>Commit Position:</span>
              <span>${this.tooltip?.commit}</span>
            </li>
            <li>
              <span>Commit Date:</span>
              <span>${this.tooltip?.date}</span>
            </li>
          </ul>
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

  private onChartMouseDown(e: MouseEvent) {
    // If the chart emits onmouoseover/out events, meaning the mouse is hovering over a data point,
    // we will not try to initiate the panning action.
    if (this.chartInteracting) {
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

  private onChartMouseOut() {
    this.chartInteracting = false;
  }

  private onWindowMouseMove(e: MouseEvent) {
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

  private onSelectDataPoint() {
    const chart = this.plotElement.value;
    if (!chart) {
      return;
    }
    const selection = chart.selection?.pop() as Selection;
    const data = chart.data as DataTableLike;
    const df = this.dataframe!;
    if (Array.isArray(data)) {
      const row = Number(selection.row);
      const column = Number(selection.column);

      this.tooltip = {
        traceName: String(data[0][column]),
        // header row is row 0, so need to add row data by 1
        value: Number(data[row + 1][column]),
        commit: String(df.header![row]?.offset),
        date: new Date(df.header![row]!.timestamp * 1000).toDateString(),
      };
    }
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
    const chart = e.detail.chart as google.visualization.CoreChartBase;
    // Only draw the anomaly when the chart is ready.
    this.drawAnomaly(chart);

    const layout = chart.getChartLayoutInterface();
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
