/**
 * @module modules/favorites-sk
 * @description <h2><code>favorites-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html } from 'lit-html';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Favorites } from '../json';
import '../window/window';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';

export class FavoritesSk extends ElementSk {
  private favoritesConfig: Favorites | null = null;

  constructor() {
    super(FavoritesSk.template);
  }

  private static template = (ele: FavoritesSk) => html`
    <header><h1 class="name">Favorites</h1></header>
    <hr />
    ${ele.getSectionsTemplate()}
  `;

  private getSectionsTemplate() {
    const sections = this.favoritesConfig?.sections;
    if (sections == null || sections.length == 0) {
      return html`No favorites have been configured for this instance.`;
    }
    return html`${sections.map(
      (section) =>
        html` <div class="section">
            <h3>${section.name}</h3>
            <table>
              <tr>
                <th>Link</th>
                <th>Description</th>
              </tr>
              ${section.links?.map(
                (link) => html`
                  <tr>
                    <td><a href=${link.href}>${link.text}</a></td>
                    <td>${link.description}</td>
                  </tr>
                `
              )}
            </table>
          </div>
          <hr />`
    )}`;
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
    if (this.favoritesConfig == null) {
      try {
        const response = await fetch('/_/favorites/');
        const json = await jsonOrThrow(response);
        this.favoritesConfig = json;
        this._render();
      } catch (error) {
        errorMessage(String(error));
      }
    }
  }
}

define('favorites-sk', FavoritesSk);
