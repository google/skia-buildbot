import { LitElement, html, css } from 'lit';
import { customElement, state, query } from 'lit/decorators.js';
import { listBenchmarks, listBots } from '../../services/api';
import '@material/web/button/filled-button.js';
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
  @state() private _benchmarks: string[] = [];

  @state() private _bots: string[] = [];

  @state() private _selectedBenchmark: string = '';

  @state() private _selectedBot: string = '';

  @state() private _selectedUser: string = '';

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

  private applyFilters() {
    this.dispatchEvent(
      new CustomEvent('filters-changed', {
        detail: {
          benchmark: this._selectedBenchmark,
          botName: this._selectedBot,
          user: this._selectedUser,
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

  render() {
    return html`
      <header>
        <h1>Pinpoint</h1>
        <div class="header-actions">
          <md-outlined-text-field
            label="Search by job name"
            @input=${this.onSearchInput}></md-outlined-text-field>
          <div style="position: relative;">
            <md-icon-button
              id="filter-anchor"
              @click=${() => {
                const menu = this.shadowRoot?.querySelector('#filter-menu') as Menu | null;
                if (menu) menu.open = !menu.open;
              }}>
              <filter-list-icon-sk></filter-list-icon-sk>
            </md-icon-button>
            <md-menu id="filter-menu" anchor="filter-anchor" stay-open-on-outside-click>
              <div class="filter-menu-items">
                <vaadin-combo-box
                  label="Benchmark"
                  .items=${this._benchmarks}
                  allow-custom-value
                  clear-button-visible
                  @value-changed=${this.onBenchmarkFilterChange}>
                </vaadin-combo-box>

                <vaadin-combo-box
                  label="Device"
                  .items=${this._bots}
                  allow-custom-value
                  clear-button-visible
                  @value-changed=${this.onBotFilterChange}>
                </vaadin-combo-box>
                <md-outlined-text-field label="User" @input=${this.onUserFilterChange}>
                </md-outlined-text-field>
                <div class="filter-actions">
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
