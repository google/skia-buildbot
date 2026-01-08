/**
 * @module modules/extra-links-sk
 * @description <h2><code>extra-links-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../window/window';

export class ExtraLinksSk extends ElementSk {
  constructor() {
    super(ExtraLinksSk.template);
  }

  private static template = (_: ExtraLinksSk) => {
    if (window.perf.extra_links) {
      return html` <header><h1 class="name">${window.perf.extra_links?.title}</h1></header>
        <hr />
        <div class="section">
          <table>
            <tr>
              <th>Link</th>
              <th>Description</th>
            </tr>
            ${window.perf.extra_links?.links?.map(
              (link) => html`
                <tr>
                  <td><a href=${link.href}>${link.text}</a></td>
                  <td>${link.description}</td>
                </tr>
              `
            )}
          </table>
        </div>`;
    } else {
      return html`No links have been configured.`;
    }
  };

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
  }
}

define('extra-links-sk', ExtraLinksSk);
