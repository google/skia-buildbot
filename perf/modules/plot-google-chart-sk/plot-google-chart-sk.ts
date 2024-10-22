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
import { Anomaly, DataFrame } from '../json';
import { convertFromDataframe, mainChartOptions } from '../common/plot-builder';
import { dataframeContext } from '../dataframe/dataframe_context';
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

export class PlotGoogleChartSk extends LitElement {
  // TODO(b/362831653): Adjust height to 100% once plot-summary-sk is deprecated
  static styles = css`
    :host {
      background-color: var(--plot-background-color-sk, var(--md-sys-color-background, 'white'));
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

  @property({ reflect: true })
  domain: 'commit' | 'date' = 'commit';

  @property({ attribute: false })
  selectedRange?: range;

  @property()
  private tooltip?: {
    traceName: string;
    value: number;
    commit: string;
    date: string;
  };

  constructor() {
    super();

    this.addEventListeners();
  }

  // The div element that will host the plot on the summary.
  private plotElement: Ref<GoogleChart> = createRef();

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
          @google-chart-select=${this.onSelectDataPoint}>
        </google-chart>
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
    `;
  }

  protected willUpdate(changedProperties: PropertyValues): void {
    if (
      // TODO(b/362831653): incorporate domain changes into dataframe update
      changedProperties.has('dataframe') ||
      changedProperties.has('selectedRange')
    ) {
      this.updateDataframe(this.dataframe!);
    }
  }

  private updateDataframe(df: DataFrame) {
    const rows = convertFromDataframe(df, this.domain);
    if (rows) {
      const plot = this.plotElement!.value!;
      plot.data = rows;
      plot.options = mainChartOptions(
        getComputedStyle(this),
        this.domain,
        titleFormatter(getTitle(this.dataframe!))
      );
    }
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

  // TODO(b/362831653): deprecate this, no longer needed
  public updateChartData(_chartData: any) {}
}

define('plot-google-chart-sk', PlotGoogleChartSk);
