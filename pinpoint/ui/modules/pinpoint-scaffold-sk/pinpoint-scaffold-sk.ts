import { LitElement, html, css } from 'lit';
import { customElement, state, query, property } from 'lit/decorators.js';
import { listBenchmarks, listBots } from '../../services/api';
import '@material/web/button/filled-button.js';
import '@material/web/button/outlined-button.js';
import '@material/web/textfield/outlined-text-field.js';
import '@material/web/iconbutton/icon-button.js';
import '@material/web/icon/icon.js';
import '../../../../elements-sk/modules/icons/filter-list-icon-sk';
import '@material/web/menu/menu.js';
import '@vaadin/combo-box/vaadin-combo-box.js';
import { Menu } from '@material/web/menu/internal/menu.js';

import '../pinpoint-new-job-sk';

import type { PinpointNewJobSk } from '../pinpoint-new-job-sk/pinpoint-new-job-sk';

/**
 * @element pinpoint-scaffold-sk
 *
 * @description Provides the main layout for the Pinpoint application.
 *
 */
@customElement('pinpoint-scaffold-sk')
export class PinpointScaffoldSk extends LitElement {
  // The properties help fill the filters box in the case where the landing
  // page pulls parameters from the URL and intially loads
  @property({ type: String }) searchTerm: string = '';

  @property({ type: String }) benchmark: string = '';

  @property({ type: String }) botName: string = '';

  @property({ type: String }) user: string = '';

  @property({ type: String }) startDate: string = '';

  @property({ type: String }) endDate: string = '';

  @state() private _benchmarks: string[] = [];

  @state() private _bots: string[] = [];

  @state() private _selectedBenchmark: string = '';

  @state() private _selectedBot: string = '';

  @state() private _selectedUser: string = '';

  @state() private _selectedStartDate: string = '';

  @state() private _selectedEndDate: string = '';

  async connectedCallback() {
    super.connectedCallback();
    this._benchmarks = await listBenchmarks();
    this._bots = await listBots(''); // Fetch all bots
  }

  static styles = css`
    :host {
      display: block;
    }
    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px;
      background-color: var(--md-sys-color-surface-container);
      border-bottom: 1px solid var(--md-sys-color-outline-variant);
    }
    h1 {
      font-size: 1.5em;
      margin: 0;
    }
    .header-actions {
      display: flex;
      gap: 16px;
      align-items: center;
    }
    main {
      padding: 16px;
    }
    .filter-menu-items {
      display: flex;
      flex-direction: column;
      gap: 16px;
      padding: 16px;
    }
    vaadin-combo-box {
      min-width: 250px;
    }
    .filter-actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      margin-top: 8px;
    }
  `;

  @query('pinpoint-new-job-sk') private _newJobModal!: PinpointNewJobSk;

  private onSearchInput(e: InputEvent) {
    const value = (e.target as HTMLInputElement).value;
    this.dispatchEvent(
      new CustomEvent('search-changed', {
        detail: { value: value },
        bubbles: true,
        composed: true,
      })
    );
  }

  private openNewJobModal() {
    this._newJobModal.show();
  }

  private onBenchmarkFilterChange(e: CustomEvent) {
    this._selectedBenchmark = e.detail.value || '';
  }

  private onBotFilterChange(e: CustomEvent) {
    this._selectedBot = e.detail.value || '';
  }

  private onUserFilterChange(e: InputEvent) {
    this._selectedUser = (e.target as HTMLInputElement).value;
  }

  private onStartDateChange(e: InputEvent) {
    this._selectedStartDate = (e.target as HTMLInputElement).value;
  }

  private onEndDateChange(e: InputEvent) {
    this._selectedEndDate = (e.target as HTMLInputElement).value;
  }

  private clearFilters() {
    this._selectedBenchmark = '';
    this._selectedBot = '';
    this._selectedUser = '';
    this._selectedStartDate = '';
    this._selectedEndDate = '';
    this.applyFilters();
  }

  private applyFilters() {
    this.dispatchEvent(
      new CustomEvent('filters-changed', {
        detail: {
          benchmark: this._selectedBenchmark,
          botName: this._selectedBot,
          user: this._selectedUser,
          startDate: this._selectedStartDate,
          endDate: this._selectedEndDate,
        },
        bubbles: true,
        composed: true,
      })
    );
    const menu = this.shadowRoot?.querySelector('#filter-menu') as Menu | null;
    if (menu) {
      menu.open = false;
    }
  }

  private openFilterMenu() {
    // When the menu is opened, initialize its state from the component's
    // current filter properties.
    this._selectedBenchmark = this.benchmark;
    this._selectedBot = this.botName;
    this._selectedUser = this.user;
    this._selectedStartDate = this.startDate;
    this._selectedEndDate = this.endDate;

    const menu = this.shadowRoot?.querySelector('#filter-menu') as Menu | null;
    if (menu) {
      menu.open = !menu.open;
    }
  }

  render() {
    return html`
      <header>
        <h1>Pinpoint</h1>
        <div class="header-actions">
          <md-outlined-text-field
            label="Search by job name"
            .value=${this.searchTerm}
            @input=${this.onSearchInput}></md-outlined-text-field>
          <div style="position: relative;">
            <md-icon-button id="filter-anchor" @click=${this.openFilterMenu}>
              <filter-list-icon-sk></filter-list-icon-sk>
            </md-icon-button>
            <md-menu id="filter-menu" anchor="filter-anchor" stay-open-on-outside-click>
              <div class="filter-menu-items">
                <vaadin-combo-box
                  label="Benchmark"
                  .items=${this._benchmarks}
                  .value=${this._selectedBenchmark}
                  allow-custom-value
                  clear-button-visible
                  @value-changed=${this.onBenchmarkFilterChange}>
                </vaadin-combo-box>

                <vaadin-combo-box
                  label="Device"
                  .items=${this._bots}
                  .value=${this._selectedBot}
                  allow-custom-value
                  clear-button-visible
                  @value-changed=${this.onBotFilterChange}>
                </vaadin-combo-box>
                <md-outlined-text-field
                  label="User"
                  .value=${this._selectedUser}
                  @input=${this.onUserFilterChange}></md-outlined-text-field>
                <md-outlined-text-field
                  label="Start Date"
                  type="date"
                  .value=${this._selectedStartDate}
                  @input=${this.onStartDateChange}>
                </md-outlined-text-field>
                <md-outlined-text-field
                  label="End Date"
                  type="date"
                  .value=${this._selectedEndDate}
                  @input=${this.onEndDateChange}>
                </md-outlined-text-field>
                <div class="filter-actions">
                  <md-outlined-button @click=${this.clearFilters}>Clear</md-outlined-button>
                  <md-filled-button @click=${this.applyFilters}>Apply</md-filled-button>
                </div>
              </div>
            </md-menu>
          </div>
          <md-filled-button @click=${this.openNewJobModal}>Create a new job</md-filled-button>
        </div>
      </header>
      <main>
        <slot></slot>
      </main>
      <pinpoint-new-job-sk></pinpoint-new-job-sk>
    `;
  }
}
