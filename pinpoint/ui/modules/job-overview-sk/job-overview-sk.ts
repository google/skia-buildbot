import { LitElement, html, css } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import { MdDialog } from '@material/web/dialog/dialog';
import '@material/web/dialog/dialog.js';
import '@material/web/button/text-button.js';
import { JobSchema } from '../../services/api';

@customElement('job-overview-sk')
export class JobOverviewSk extends LitElement {
  @property({ type: Object }) job: JobSchema | null = null;

  @query('md-dialog') private _dialog!: MdDialog;

  static styles = css`
    .params-table {
      width: 100%;
      border-collapse: collapse;
    }
    .params-table th,
    .params-table td {
      text-align: left;
      padding: 8px;
      border-bottom: 1px solid var(--md-sys-color-outline-variant);
      vertical-align: top;
    }
    .params-table th {
      width: 30%;
      font-weight: bold;
    }
    pre {
      margin: 0;
      white-space: pre-wrap;
      word-wrap: break-word;
    }
  `;

  public show() {
    this._dialog.show();
  }

  private renderParameterValue(value: any): any {
    if (typeof value === 'object' && value !== null) {
      return html`<pre>${JSON.stringify(value, null, 2)}</pre>`;
    }
    return value;
  }

  private formatKey(key: string): string {
    const keyMap: { [key: string]: string } = {
      start_commit_githash: 'Start Commit',
      end_commit_githash: 'End Commit',
      bug_id: 'Bug ID',
      initial_attempt_count: 'Attempt Count',
      improvement_direction: 'Improvement Direction',
      story_tags: 'Story Tags',
      aggregation_method: 'Aggregation Method',
    };
    if (keyMap[key]) {
      return keyMap[key];
    }
    // Fallback for other keys: replace underscores and capitalize.
    return key
      .split('_')
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ');
  }

  render() {
    if (!this.job) {
      return html``;
    }

    // Combine top-level and additional parameters for display.
    const paramsToShow = [
      ['Benchmark', this.job.Benchmark],
      ['Bot Configuration', this.job.BotName],
      ...Object.entries(this.job.AdditionalRequestParameters).filter(
        ([key]) => key !== 'commit_runs' && key !== 'duration'
      ),
    ];

    return html`
      <md-dialog>
        <div slot="headline">Job Arguments</div>
        <div slot="content">
          <table class="params-table">
            <tbody>
              ${paramsToShow.map(
                ([key, value]) =>
                  html` <tr>
                    <th>${this.formatKey(key as string)}</th>
                    <td>${this.renderParameterValue(value)}</td>
                  </tr>`
              )}
            </tbody>
          </table>
        </div>
        <div slot="actions">
          <md-text-button @click=${() => this._dialog.close()}>Close</md-text-button>
        </div>
      </md-dialog>
    `;
  }
}
