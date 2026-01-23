import { html, LitElement } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { RegressionBug } from '../json';

@customElement('bug-tooltip-sk')
export class BugTooltipSk extends LitElement {
  @property({ type: Array })
  bugs: RegressionBug[] = [];

  @property({ type: String })
  totalLabel: string = 'total';

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <div class="bug-count-container" ?hidden=${this.bugs.length === 0}>
        <span>with ${this.bugs.length} ${this.totalLabel}</span>
        <div class="bug-tooltip">
          ${html`
            <ul>
              ${this.bugs.map(
                (bug) =>
                  html`<li>
                    <a href="http://b/${bug.bug_id}" target="_blank">${bug.bug_id}</a
                    ><span> from ${bug.bug_type}</span>
                  </li>`
              )}
            </ul>
          `}
        </div>
      </div>
    `;
  }
}
