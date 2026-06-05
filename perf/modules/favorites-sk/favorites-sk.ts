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
import { html, LitElement } from 'lit';
import { customElement } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { repeat } from 'lit/directives/repeat.js';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Favorites } from '../json';
import '../window/window';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import '../../../elements-sk/modules/icons/delete-icon-sk';
import '../favorites-dialog-sk';
import { FavoritesDialogSk } from '../favorites-dialog-sk/favorites-dialog-sk';
import { $$ } from '../../../infra-sk/modules/dom';

@customElement('favorites-sk')
export class FavoritesSk extends LitElement {
  private _fetchTask = new Task(this, {
    task: async ([], { signal }) => {
      const response = await fetch('/_/favorites/', { signal });
      const json = await jsonOrThrow(response);
      return json as Favorites;
    },
    args: () => [] as const,
  });

  createRenderRoot() {
    return this;
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

    this._fetchTask.run();
  };

  private deleteFavoriteConfirm = async (id: string | undefined, name: string) => {
    if (id === undefined) return;

    const confirmed = window.confirm(`Deleting favorite: ${name}. Are you sure?`);
    if (!confirmed) {
      return;
    }

    await this.deleteFavorite(id);
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
        this._fetchTask.run();
      })
      .catch((e) => {
        if (e !== undefined) {
          errorMessage(`${e}`);
        }
      });
  };

  render() {
    return html`
      <header><h1 class="name">Favorites</h1></header>
      <hr />
      ${this._fetchTask.render({
        pending: () => html`<div>Loading favorites...</div>`,
        error: (e) => html`<div>Error loading favorites: ${e}</div>`,
        complete: (favoritesConfig) => this.getSectionsTemplate(favoritesConfig),
      })}
    `;
  }

  private getSectionsTemplate(favoritesConfig: Favorites) {
    const sections = favoritesConfig.sections;
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
              ${repeat(
                section.links || [],
                (link, index) => link.id || `${link.text}-${index}`,
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
            ${repeat(
              section.links || [],
              (link, index) => link.id || `${link.text}-${index}`,
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
}
