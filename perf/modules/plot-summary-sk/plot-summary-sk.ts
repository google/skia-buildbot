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
import { PlotSelectionEventDetails } from '../plot-google-chart-sk/plot-google-chart-sk';

import { defaultColors, SummaryChartOptions } from '../common/plot-builder';
import { ColumnHeader } from '../json';
import {
  dataframeLoadingContext,
  dataframeRepoContext,
  DataFrameRepository,
  DataTable,
  dataTableContext,
} from '../dataframe/dataframe_context';
import { range } from '../dataframe/index';
import { define } from '../../../elements-sk/modules/define';
import { SummaryMetric, telemetry } from '../telemetry/telemetry';

import { style } from './plot-summary-sk.css';
import { HResizableBoxSk } from './h_resizable_box_sk';

export interface PlotSummarySkSelectionEventDetails {
  start: number;
  end: number;
  value: range;
  domain: 'commit' | 'date';
  graphNumber?: number; // optional argument used to sync multi-graphs
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

  @property({ attribute: true })
  selectedTrace: string | null = null;

  // load ~1Q worth of data as developer cycles tend to
  // revolve around quarterly milestones. Users are likely
  // to want to see historical data in this increment.
  @property({ type: Number })
  loadingChunk = 90 * dayInSeconds;

  @property({ type: Boolean })
  hasControl: boolean = false;

  // Maps a trace to a color.
  private traceColorMap = new Map<string, string>();

  // Index to keep track of which colors we've used so far.
  private colorIndex = 0;

  constructor() {
    super();
    this.addEventListeners();
  }

  protected updated(changedProperties: PropertyValues): void {
    if (
      changedProperties.has('data') ||
      changedProperties.has('selectedTrace') ||
      changedProperties.has('domain')
    ) {
      this.updateDataView(this.data, this.selectedTrace);
    }
  }

  private updateDataView(dt: DataTable, trace: string | null) {
    const start = performance.now();
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
    const options = SummaryChartOptions(getComputedStyle(this), this.domain);
    options.colors = [];

    // Downsample the data if there are too many points.
    const numRows = dt.getNumberOfRows();
    const maxPoints = 1000;
    if (numRows > maxPoints) {
      const sampleRate = Math.ceil(numRows / maxPoints);
      const rowsToKeep = [];
      for (let i = 0; i < numRows; i += sampleRate) {
        rowsToKeep.push(i);
      }
      view.setRows(rowsToKeep);
    }

    // The first two columns are the commit position and the date.
    const cols = [this.domain === 'commit' ? 0 : 1];
    for (let i = 2; i < dt.getNumberOfColumns(); i++) {
      const traceKey = dt.getColumnLabel(i);

      // Assign a specific color to all labels if not already present.
      if (!this.traceColorMap.has(traceKey)) {
        this.traceColorMap.set(traceKey, defaultColors[this.colorIndex % defaultColors.length]);
        this.colorIndex++;
      }

      if (!trace || trace === traceKey) {
        cols.push(i);
        options.colors.push(this.traceColorMap.get(traceKey)!);
      }
    }

    view.setColumns(cols);
    plot.view = view;
    plot.options = options;
    telemetry.recordSummary(SummaryMetric.GoogleGraphPlotTime, (performance.now() - start) / 1000, {
      type: 'summary',
    });
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
      // Check if multichart is enabled.
      const isMultiChart = document.querySelectorAll('explore-simple-sk').length > 1 ? true : false;

      if (isMultiChart) {
        // If the multi-chart is enabled, we need to update the selection in all charts.
        // When extending the range, we need to update other charts to match.
        const detail: PlotSelectionEventDetails = {
          value: this.dfRepo?.commitRange ?? range(0, 0),
          domain: this.domain,
          offsetInSeconds: chunk * direction,
        };
        this.dispatchEvent(
          new CustomEvent<PlotSelectionEventDetails>('range-changing-in-multi', {
            bubbles: true,
            detail: detail,
          })
        );
      } else {
        await this.dfRepo?.extendRange(chunk * direction);
      }
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
        <h-resizable-box-sk
          ${ref(this.selectionBox)}
          @selection-changed=${this.onSelectionChanged}></h-resizable-box-sk>
        ${when(
          this.loading,
          () =>
            html`<div class="overlay">
              <div class="loader"></div>
            </div>`
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
      return;
    }
    this.selectedValueRange = this.cachedSelectedValueRange;
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
    if (this.cachedSelectedValueRange) {
      return this.cachedSelectedValueRange;
    }
    const chart = this.chartLayout;
    if (!chart) {
      return { begin: 0, end: 0 };
    }

    const coordsRange = this.selectionBox.value?.selectionRange || null;
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
  }

  // SelectRange sets the selection range on the plot summary bar.
  public SelectRange(selectionRange: range) {
    this.selectedValueRange = selectionRange;
  }

  // Select the provided range on the plot-summary.
  public Select(begin: ColumnHeader, end: ColumnHeader) {
    const isCommitScale = this.domain === 'commit';
    const col = isCommitScale ? 'offset' : 'timestamp';
    this.SelectRange({ begin: begin[col], end: end[col] });
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
