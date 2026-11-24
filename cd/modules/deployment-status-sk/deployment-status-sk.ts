import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

type ColumnKey =
  | 'cluster'
  | 'namespace'
  | 'pod'
  | 'container'
  | 'commit'
  | 'commit_time'
  | 'commit_subject'
  | 'image';

interface Entry {
  cluster: string;
  namespace: string;
  pod: string;
  container: string;
  commit: string;
  commit_time: string;
  commit_subject: string;
  image: string;
  is_ancestor?: boolean;
}

interface EntriesResponse {
  entries: Entry[];
  total: number;
  last_updated?: string;
  refresh_error?: string;
  filter_errors?: Record<string, string>;
  ancestor_error?: string;
}

interface ColumnDef {
  key: ColumnKey;
  label: string;
  mono?: boolean;
}

const COLUMNS: ColumnDef[] = [
  { key: 'cluster', label: 'Cluster' },
  { key: 'namespace', label: 'Namespace' },
  { key: 'pod', label: 'Pod', mono: true },
  { key: 'container', label: 'Container' },
  { key: 'commit', label: 'Commit', mono: true },
  { key: 'commit_time', label: 'Commit Time', mono: true },
  { key: 'commit_subject', label: 'Commit Subject' },
  { key: 'image', label: 'Image', mono: true },
];

const MAX_SUBJECT_LENGTH = 60;
const DEBOUNCE_DELAY_MS = 300;

export class DeploymentStatusSk extends ElementSk {
  private currentData: EntriesResponse = {
    entries: [],
    total: 0,
  };

  private filters: Record<ColumnKey, string> = {
    cluster: '',
    namespace: '',
    pod: '',
    container: '',
    commit: '',
    commit_time: '',
    commit_subject: '',
    image: '',
  };

  private ancestor: string = '';

  private ignoreExternal: boolean = true;

  private sortCol: ColumnKey = 'cluster';

  private sortAsc: boolean = true;

  private activeController: AbortController | null = null;

  private debounceTimer: number | null = null;

  constructor() {
    super(DeploymentStatusSk.template);
  }

  private static template = (el: DeploymentStatusSk): TemplateResult => {
    const entries = el.getSortedEntries();
    const invalidCols = el.currentData.filter_errors || {};
    const ancestorError = el.currentData.ancestor_error || '';
    const errorMessages: string[] = [];
    if (el.currentData.refresh_error) {
      errorMessages.push(el.currentData.refresh_error);
    }
    if (ancestorError) {
      errorMessages.push(ancestorError);
    }
    for (const col of COLUMNS) {
      if (invalidCols[col.key]) {
        errorMessages.push(`Invalid ${col.label} filter: ${invalidCols[col.key]}`);
      }
    }
    const updatedText = el.currentData.last_updated
      ? ` (updated ${new Date(el.currentData.last_updated).toLocaleString()})`
      : '';
    return html`
      <header>
        <h1>Deployment Status</h1>
        <div class="controls">
          <span class="status-text">
            Showing ${entries.length} of ${el.currentData.total || 0} containers${updatedText}
          </span>
          <label class="checkbox-label">
            <input
              type="checkbox"
              .checked=${el.ignoreExternal}
              @change=${(e: Event) => {
                el.ignoreExternal = (e.target as HTMLInputElement).checked;
                el.onFilterChange();
              }} />
            Ignore external images
          </label>
          <input
            class="ancestor-input ${ancestorError ? 'invalid' : ''}"
            placeholder="Check commit or CL..."
            title=${ancestorError || ''}
            .value=${el.ancestor}
            @input=${(e: InputEvent) => {
              el.ancestor = (e.target as HTMLInputElement).value;
              el.onFilterChange();
            }} />
          <button type="button" @click=${() => el.clearFilters()}>Clear Filters</button>
        </div>
      </header>

      ${errorMessages.map((msg) => html`<div class="error-banner">${msg}</div>`)}

      <div class="table-container">
        <table>
          <thead>
            <tr>
              ${COLUMNS.map(
                (col) => html`
                  <th class="col-header" @click=${() => el.toggleSort(col.key)}>
                    ${col.label}<span class="sort-indicator"
                      >${el.sortCol === col.key ? (el.sortAsc ? '\u25B2' : '\u25BC') : ''}</span
                    >
                  </th>
                `
              )}
            </tr>
            <tr class="filter-row">
              ${COLUMNS.map((col) => {
                const errMsg = invalidCols[col.key] || '';
                return html`
                  <th>
                    <input
                      class="filter-input ${errMsg ? 'invalid' : ''}"
                      placeholder="Regex filter..."
                      title=${errMsg}
                      .value=${el.filters[col.key]}
                      @input=${(e: InputEvent) => {
                        el.filters[col.key] = (e.target as HTMLInputElement).value;
                        el.onFilterChange();
                      }} />
                  </th>
                `;
              })}
            </tr>
          </thead>
          <tbody>
            ${entries.map((entry) => el.renderRow(entry))}
          </tbody>
        </table>
      </div>
    `;
  };

  connectedCallback(): void {
    super.connectedCallback();
    this.initFromURL();
    this._render();
    this.fetchFiltered().catch((e) => {
      if (e.name !== 'AbortError') {
        console.error('Initial fetch failed:', e);
      }
    });
  }

  private initFromURL(): void {
    const urlParams = new URLSearchParams(window.location.search);
    for (const col of COLUMNS) {
      const val = urlParams.get(col.key);
      if (val) {
        this.filters[col.key] = val;
      }
    }
    const ancestorParam = urlParams.get('ancestor');
    if (ancestorParam) {
      this.ancestor = ancestorParam;
    }
    this.ignoreExternal = urlParams.get('ignore_external') !== 'false';
  }

  private buildQueryParams(): URLSearchParams {
    const params = new URLSearchParams();
    for (const col of COLUMNS) {
      const val = this.filters[col.key];
      if (val) {
        params.set(col.key, val);
      }
    }
    if (this.ancestor.trim()) {
      params.set('ancestor', this.ancestor.trim());
    }
    if (!this.ignoreExternal) {
      params.set('ignore_external', 'false');
    }
    return params;
  }

  private syncURLParams(): void {
    const params = this.buildQueryParams();
    const qs = params.toString();
    const newURL = window.location.pathname + (qs ? `?${qs}` : '');
    window.history.replaceState(null, '', newURL);
  }

  private async fetchFiltered(): Promise<void> {
    if (this.activeController) {
      this.activeController.abort();
    }
    const controller = new AbortController();
    this.activeController = controller;

    const params = this.buildQueryParams();
    const qs = params.toString();
    const resp = await fetch(`/api/entries${qs ? `?${qs}` : ''}`, {
      signal: controller.signal,
    });
    if (!resp.ok) {
      throw new Error(await resp.text());
    }
    this.currentData = (await resp.json()) as EntriesResponse;
    this._render();
  }

  private getSortedEntries(): Entry[] {
    return [...(this.currentData.entries || [])].sort((a, b) => {
      const va = a[this.sortCol] || '';
      const vb = b[this.sortCol] || '';
      const cmp = va.localeCompare(vb);
      return this.sortAsc ? cmp : -cmp;
    });
  }

  private renderRow(entry: Entry): TemplateResult {
    let rowClass = '';
    if (entry.is_ancestor === true) {
      rowClass = 'ancestor-yes';
    } else if (entry.is_ancestor === false) {
      rowClass = 'ancestor-no';
    }

    return html`
      <tr class=${rowClass}>
        ${COLUMNS.map((col) => {
          const val = entry[col.key] || '';
          let displayVal = val;
          let title = '';
          if (col.key === 'commit' && val.length > 7) {
            displayVal = val.slice(0, 7);
            title = val;
          } else if (col.key === 'commit_subject' && val.length > MAX_SUBJECT_LENGTH) {
            displayVal = `${val.slice(0, MAX_SUBJECT_LENGTH - 3)}...`;
            title = val;
          }
          return html`<td class=${col.mono ? 'mono' : ''} title=${title}>${displayVal}</td>`;
        })}
      </tr>
    `;
  }

  private onFilterChange(): void {
    this.syncURLParams();
    this._render();
    if (this.debounceTimer !== null) {
      window.clearTimeout(this.debounceTimer);
    }
    this.debounceTimer = window.setTimeout(() => {
      this.fetchFiltered().catch((e) => {
        if (e.name !== 'AbortError') {
          console.error('Filter request failed:', e);
        }
      });
    }, DEBOUNCE_DELAY_MS);
  }

  private toggleSort(col: ColumnKey): void {
    if (this.sortCol === col) {
      this.sortAsc = !this.sortAsc;
    } else {
      this.sortCol = col;
      this.sortAsc = true;
    }
    this._render();
  }

  private clearFilters(): void {
    for (const col of COLUMNS) {
      this.filters[col.key] = '';
    }
    this.ancestor = '';
    this.ignoreExternal = true;
    this.onFilterChange();
  }
}

define('deployment-status-sk', DeploymentStatusSk);
