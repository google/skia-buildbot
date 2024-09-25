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

export interface AnomalyData {
  x: number;
  y: number;
  anomaly: Anomaly;
  highlight: boolean;
}

export class PlotGoogleChartSk extends LitElement {
  // TODO(b/362831653): Adjust height to 100% once plot-summary-sk is deprecated
  static styles = css`
    :host {
      display: block;
      background-color: var(
        --plot-background-color-sk,
        var(--md-sys-color-background, 'white')
      );
    }
    .plot {
      position: absolute;
      top: 200;
      left: 0;
      width: 100%;
      height: 40%;
    }
  `;

  @consume({ context: dataframeContext, subscribe: true })
  @property({ attribute: false })
  private dataframe?: DataFrame;

  @property({ reflect: true })
  domain: 'commit' | 'date' = 'commit';

  constructor() {
    super();

    this.addEventListeners();
  }

  // The div element that will host the plot on the summary.
  private plotElement: Ref<GoogleChart> = createRef();

  connectedCallback(): void {
    super.connectedCallback();

    const resizeObserver = new ResizeObserver(
      (entries: ResizeObserverEntry[]) => {
        entries.forEach(() => {
          // The google chart needs to redraw when it is resized.
          this.plotElement.value?.redraw();
        });
      }
    );
    resizeObserver.observe(this);
  }

  protected render() {
    return html`
      <google-chart ${ref(this.plotElement)} class="plot" type="line">
      </google-chart>
    `;
  }

  protected willUpdate(changedProperties: PropertyValues): void {
    if (
      // TODO(b/362831653): incorporate domain changes into dataframe update
      changedProperties.has('dataframe')
    ) {
      this.updateDataframe(this.dataframe!);
    }
  }

  private updateDataframe(df: DataFrame) {
    const rows = convertFromDataframe(df, this.domain);
    if (rows) {
      const plot = this.plotElement!.value!;
      plot.data = rows;
      // TODO(b/362831653): add event listener for dark mode
      plot.options = mainChartOptions(getComputedStyle(this), this.domain);
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
          this.domain
        );
      }
      this.requestUpdate();
    });
  }

  // TODO(b/362831653): deprecate this, no longer needed
  public updateChartData(_chartData: any) {}
}

define('plot-google-chart-sk', PlotGoogleChartSk);
