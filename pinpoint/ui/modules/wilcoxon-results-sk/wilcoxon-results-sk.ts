import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { JobSchema, WilcoxonResult } from '../../services/api';

@customElement('wilcoxon-result-sk')
export class WilcoxonResultSk extends LitElement {
  @property({ attribute: false })
  job: JobSchema | null = null;

  static styles = css`
    .result-table {
      width: 100%;
      border-collapse: collapse;
      margin-top: 1em;
      font-size: 0.9em;
    }
    .result-table th,
    .result-table td {
      border: 1px solid #dadce0;
      padding: 8px;
      text-align: left;
      vertical-align: middle;
    }
    .result-table th {
      background-color: #f1f3f4;
      font-weight: bold;
    }
    .value-cell {
      padding: 4px 8px;
      border-radius: 4px;
      display: inline-block;
      min-width: 80px;
      text-align: center;
    }
    .improvement {
      background-color: lightgreen;
    }
    .regression {
      background-color: lightcoral;
    }
    .neutral {
      background-color: lightgrey;
    }
    .arrow {
      font-size: 1.2em;
      line-height: 1;
    }
  `;

  private renderImprovementArrow(direction: string) {
    if (direction === 'UP') {
      return html`<span class="arrow" title="Higher is better">&#8599;</span>`;
    }
    if (direction === 'DOWN') {
      return html`<span class="arrow" title="Lower is better">&#8600;</span>`;
    }
    return html`<span title="Improvement direction unknown">-</span>`;
  }

  private calculatePercentageChange(control: number, treatment: number): string {
    if (control === 0) {
      return 'N/A';
    }
    const change = ((treatment - control) / control) * 100;
    return `${change.toFixed(2)}%`;
  }

  private formatConfidenceInterval(result: WilcoxonResult): string {
    if (result.control_median === 0) {
      return `[${result.confidence_interval_lower.toExponential(
        2
      )}, ${result.confidence_interval_higher.toExponential(2)}] (abs)`;
    }
    const lower = (result.confidence_interval_lower / result.control_median) * 100;
    const higher = (result.confidence_interval_higher / result.control_median) * 100;
    return `[${lower.toFixed(2)}%, ${higher.toFixed(2)}%]`;
  }

  private getValueClass(result: WilcoxonResult, direction: string): string {
    if (!result.significant) {
      return 'neutral';
    }
    if (direction === 'UNKNOWN') {
      return 'neutral';
    }

    const delta = result.treatment_median - result.control_median;
    if (delta === 0) {
      return 'neutral';
    }

    const isImprovement = (direction === 'UP' && delta > 0) || (direction === 'DOWN' && delta < 0);

    return isImprovement ? 'improvement' : 'regression';
  }

  render() {
    if (!this.job || !this.job.MetricSummary || Object.keys(this.job.MetricSummary).length === 0) {
      return html`<p>No Wilcoxon results available for this job.</p>`;
    }

    // TODO(b/440108016) Implement improvement direction in backend
    // For now default to UP
    const effectiveDir = 'UP';
    const charts = Object.keys(this.job.MetricSummary);

    return html`
      <h3>Pairwise Wilcoxon Results</h3>
      <table class="result-table">
        <thead>
          <tr>
            <th>Measurement</th>
            <th>Improvement</th>
            <th>% Median Diff</th>
            <th>95% CI (% of Median)</th>
            <th>P-Value</th>
          </tr>
        </thead>
        <tbody>
          ${charts.map((chart) => {
            const result = this.job!.MetricSummary[chart];
            const valueClass = this.getValueClass(result, effectiveDir);
            return html`
              <tr>
                <td>${chart}</td>
                <td>${this.renderImprovementArrow(effectiveDir)}</td>
                <td>
                  <div class="value-cell ${valueClass}">
                    ${this.calculatePercentageChange(
                      result.control_median,
                      result.treatment_median
                    )}
                  </div>
                </td>
                <td>
                  <div class="value-cell ${valueClass}">
                    ${this.formatConfidenceInterval(result)}
                  </div>
                </td>
                <td>
                  <div class="value-cell ${valueClass}">${result.p_value.toPrecision(3)}</div>
                </td>
              </tr>
            `;
          })}
        </tbody>
      </table>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'wilcoxon-result-sk': WilcoxonResultSk;
  }
}
