import { LitElement, html, css } from 'lit';
import { customElement } from 'lit/decorators.js';
import '@material/web/button/filled-button.js';
import '@material/web/textfield/outlined-text-field.js';

/**
 * @element pinpoint-scaffold-sk
 *
 * @description Provides the main layout for the Pinpoint application.
 *
 */

@customElement('pinpoint-scaffold-sk')
export class PinpointScaffoldSk extends LitElement {
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
  `;

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

  render() {
    return html`
      <header>
        <h1>Pinpoint</h1>
        <div class="header-actions">
          <md-outlined-text-field
            label="Search by job name"
            @input=${this.onSearchInput}></md-outlined-text-field>
          <md-filled-button>Create new job</md-filled-button>
        </div>
      </header>
      <main>
        <slot></slot>
      </main>
    `;
  }
}
