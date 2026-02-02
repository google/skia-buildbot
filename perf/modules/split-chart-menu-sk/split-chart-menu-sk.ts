/**
 * @module modules/split-chart-menu-sk
 * @description <h2><code>split-chart-menu-sk</code></h2>
 *
 * SplitChartMenuSk is a menu that contains the options for
 * splitting the chart by attribute. Example attributes are
 * benchmark, story, subtest, etc.
 *
 */
import { consume } from '@lit/context';
import { html, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { dataframeContext, DataTable, dataTableContext } from '../dataframe/dataframe_context';
import { getAttributes } from '../dataframe/traceset';
import { define } from '../../../elements-sk/modules/define';

import { style } from './split-chart-menu-sk.css';
import { DataFrame } from '../json';

import '@material/web/menu/menu.js';
import '@material/web/menu/menu-item.js';

export interface SplitChartSelectionEventDetails {
  attribute: string;
}

// DEPRECATED in favor of Split Checkboxes.
export class SplitChartMenuSk extends LitElement {
  static styles = style;

  @consume({ context: dataframeContext, subscribe: true })
  @property({ attribute: false })
  private df?: DataFrame;

  @consume({ context: dataTableContext, subscribe: true })
  @property({ attribute: false })
  data: DataTable = null;

  @state()
  menuOpen = false;

  private menuClicked(e: Event) {
    e.preventDefault();
    e.stopPropagation();
    this.menuOpen = !this.menuOpen;
  }

  private menuClosed() {
    this.menuOpen = false;
  }

  protected render() {
    return html`
      <md-outlined-button id="menu-id" @click=${this.menuClicked}> Split By </md-outlined-button>
      <md-menu
        .open=${this.menuOpen}
        anchor="menu-id"
        positioning="fixed"
        quick="true"
        @closed=${this.menuClosed}>
        ${this.getAttributes().map(
          (attr) => html`
            <md-menu-item
              @click=${() => {
                this.bubbleAttribute(attr);
              }}>
              ${attr}
            </md-menu-item>
          `
        )}
      </md-menu>
    `;
  }

  private bubbleAttribute(attribute: string) {
    this.dispatchEvent(
      new CustomEvent<SplitChartSelectionEventDetails>('split-chart-selection', {
        bubbles: true,
        composed: true,
        detail: {
          attribute: attribute,
        },
      })
    );
  }

  private getAttributes(): string[] {
    if (!this.df) {
      return [];
    }

    return getAttributes(this.df);
  }
}

define('split-chart-menu-sk', SplitChartMenuSk);
