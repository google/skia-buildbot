import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { Job } from '../../services/api';
import '@material/web/icon/icon.js';
import '../../../../elements-sk/modules/icons/arrow-upward-icon-sk';
import '../../../../elements-sk/modules/icons/arrow-downward-icon-sk';

type SortDirection = 'asc' | 'desc';

interface Column {
  key: string;
  label: string;
}

/**
 * @element jobs-table-sk
 *
 * @description Displays a sortable table of Pinpoint jobs.
 *
 * @fires sort-changed - When the user clicks a column header to change the sort order.
 */
@customElement('jobs-table-sk')
export class JobsTableSk extends LitElement {
  @property({ type: Array }) jobs: Job[] = [];

  @property({ type: String }) sortBy: string = 'created_date';

  @property({ type: String }) sortDir: SortDirection = 'desc';

  private columns: Column[] = [
    { key: 'job_name', label: 'Job Name' },
    { key: 'benchmark', label: 'Benchmark' },
    { key: 'bot_name', label: 'Bot Name' },
    { key: 'user', label: 'User' },
    { key: 'created_date', label: 'Created' },
    { key: 'job_type', label: 'Type' },
    { key: 'job_status', label: 'Status' },
  ];

  static styles = css`
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.9em;
    }
    th,
    td {
      padding: 12px 15px;
      text-align: left;
      border-bottom: 1px solid var(--md-sys-color-outline-variant);
    }
    th {
      cursor: pointer;
      user-select: none;
    }
    th:hover {
      background-color: var(--md-sys-color-surface-container-highest);
    }
    .sort-icon {
      vertical-align: middle;
      font-size: 1.2em;
    }
  `;

  private onHeaderClick(key: string) {
    let newDir: SortDirection = 'desc';
    if (this.sortBy === key) {
      if (this.sortDir === 'desc') {
        newDir = 'asc';
      } else {
        newDir = 'desc';
      }
    }
    this.dispatchEvent(
      new CustomEvent('sort-changed', {
        detail: { sortBy: key, sortDir: newDir },
      })
    );
  }

  private renderSortIcon(key: string) {
    if (this.sortBy !== key) return '';

    if (this.sortDir === 'desc') {
      return html`<arrow-downward-icon-sk></arrow-downward-icon-sk>`;
    } else {
      return html`<arrow-upward-icon-sk></arrow-upward-icon-sk>`;
    }
  }

  private formatDate(dateStr: string): string {
    if (!dateStr) {
      return '';
    }

    return new Intl.DateTimeFormat(navigator.language, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    }).format(new Date(dateStr));
  }

  render() {
    return html`
      <table>
        <thead>
          <tr>
            ${this.columns.map(
              (col) => html`
                <th @click=${() => this.onHeaderClick(col.key)}>
                  ${col.label} ${this.renderSortIcon(col.key)}
                </th>
              `
            )}
          </tr>
        </thead>
        <tbody>
          ${this.jobs.map(
            (job) => html`
              <tr>
                <td>${job.job_name}</td>
                <td>${job.benchmark}</td>
                <td>${job.bot_name}</td>
                <td>${job.user}</td>
                <td>${this.formatDate(job.created_date)}</td>
                <td>${job.job_type}</td>
                <td>${job.job_status}</td>
              </tr>
            `
          )}
        </tbody>
      </table>
    `;
  }
}
