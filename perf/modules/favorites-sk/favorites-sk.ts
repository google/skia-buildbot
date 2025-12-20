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
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Favorites } from '../json';
import '../window/window';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import '../../../elements-sk/modules/icons/delete-icon-sk';
import '../favorites-dialog-sk';
import { FavoritesDialogSk } from '../favorites-dialog-sk/favorites-dialog-sk';
import { $$ } from '../../../infra-sk/modules/dom';

export class FavoritesSk extends ElementSk {
  private favoritesConfig: Favorites | null = null;

  constructor() {
    super(FavoritesSk.template);
  }

  private deleteFavorite = async (favId: string) => {
    const body = {
      id: favId,
    };
    const resp = await fetch('/_/favorites/delete', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    });
    if (!resp.ok) {
      const msg = await resp.text();
      errorMessage(`${resp.statusText}: ${msg}`);
      return;
    }

    await this.fetchFavorites();
  };

  private deleteFavoriteConfirm = async (id: string | undefined, name: string) => {
    if (id === undefined) return;

    const confirmed = window.confirm(`Deleting favorite: ${name}. Are you sure?`);
    if (!confirmed) {
      return;
    }

    this.deleteFavorite(id);
  };

  private editFavorite = async (
    id: string | undefined,
    name: string,
    desc: string,
    url: string
  ) => {
    const d = $$<FavoritesDialogSk>('#fav-dialog', this) as FavoritesDialogSk;

    d!
      .open(id, name, desc, url)
      .then(() => {
        this.fetchFavorites();
      })
      .catch((e) => {
        if (e !== undefined) {
          errorMessage(`${e}`);
        }
      });
  };

  private static template = (ele: FavoritesSk) => html`
    <header><h1 class="name">Favorites</h1></header>
    <hr />
    ${ele.getSectionsTemplate()}
  `;

  private getSectionsTemplate() {
    const sections = this.favoritesConfig?.sections;
    // eslint-disable-next-line eqeqeq
    if (sections == null || sections.length === 0) {
      return html`No favorites have been configured for this instance.`;
    }
    return html`${sections.map((section) => {
      if (section.name === 'My Favorites') {
        return html` <div class="section">
            <h3>${section.name}</h3>
            <table>
              <tr>
                <th>Link</th>
                <th>Description</th>
                <th>Actions</th>
              </tr>
              ${section.links?.map(
                (link) => html`
                  <tr>
                    <td><a href=${link.href}>${link.text}</a></td>
                    <td>${link.description}</td>
                    <td>
                      <button
                        class="edit-favorite"
                        @click=${() =>
                          this.editFavorite(link.id, link.text, link.description, link.href)}>
                        Edit
                      </button>
                      <button
                        class="delete-favorite"
                        @click=${() => this.deleteFavoriteConfirm(link.id, link.text)}>
                        Delete
                      </button>
                    </td>
                  </tr>
                `
              )}
            </table>
            <favorites-dialog-sk id="fav-dialog"></favorites-dialog-sk>
          </div>
          <hr />`;
      }

      return html` <div class="section">
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
        <hr />`;
    })}`;
  }

  private fetchFavorites = async () => {
    const response = await fetch('/_/favorites/');
    const json = await jsonOrThrow(response);
    this.favoritesConfig = json;
    this._render();
  };

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
    if (this.favoritesConfig === null) {
      this.fetchFavorites().catch(errorMessage);
    }
  }
}

define('favorites-sk', FavoritesSk);
