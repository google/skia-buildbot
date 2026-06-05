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
import { html, LitElement } from 'lit';
import { customElement } from 'lit/decorators.js';
import '../window/window';

@customElement('extra-links-sk')
export class ExtraLinksSk extends LitElement {
  createRenderRoot() {
    return this;
  }

  render() {
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
  }
}
