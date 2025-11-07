/**
 * @module modules/favorites-dialog-sk
 * @description <h2><code>favorites-dialog-sk</code></h2>
 *
 * This module is a modal that contains a form to capture user
 * input for adding/editing a new favorite.
 */
import { html } from 'lit/html.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { $$ } from '../../../infra-sk/modules/dom';
import { define } from '../../../elements-sk/modules/define';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/icons/close-icon-sk';

// FavoritesDialogSk is a modal that contains a form to capture user
// input for adding/editing a new favorite.
export class FavoritesDialogSk extends ElementSk {
  private static nextUniqueId = 0;

  private readonly uniqueId = `${FavoritesDialogSk.nextUniqueId++}`;

  favId: string = '';

  name: string = '';

  description: string = '';

  url: string = '';

  private dialog: HTMLDialogElement | null = null;

  private updatingFavorite: boolean = false;

  private resolve: ((value?: any) => void) | null = null;

  private reject: ((value?: any) => void) | null = null;

  constructor() {
    super(FavoritesDialogSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = $$('dialog', this);
  }

  private dismiss(): void {
    this.dialog!.close();
    this.reject!();
  }

  private async confirm(): Promise<void> {
    if (this.name === '' || this.url === '') {
      errorMessage('Name and url must be non empty');
      return;
    }

    let apiUrl = '/_/favorites/new';
    let body: {
      id?: string;
      name: string;
      description: string;
      url: string;
    } = {
      name: this.name,
      description: this.description,
      url: this.url,
    };
    if (this.favId !== '') {
      body = { ...body, id: this.favId };
      apiUrl = '/_/favorites/edit';
    }

    try {
      this.updatingFavorite = true;
      this._render();
      const resp = await fetch(apiUrl, {
        method: 'POST',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!resp.ok) {
        const msg = await resp.text();
        errorMessage(`${resp.statusText}: ${msg}`);
      }
    } finally {
      this.updatingFavorite = false;
      this.dialog!.close();
      this.resolve!();
    }
  }

  // open shows the popup dialog when called.
  public open(favId?: string, name?: string, description?: string, url?: string): Promise<void> {
    this.favId = favId || '';
    this.name = name || '';
    this.description = description || '';
    this.url = url || window.location.href;

    this._render();
    this.dialog!.showModal();

    // If the dialog closes it could be due to 2 reasons:
    //    1: User pressed on close
    //    2: The favorite got added/edited.
    // In this module, we want to re-fetch the favorites when the dialog is closed
    // but we only want to re-fetch if closed due to reason 2.
    // So we're using the reject function when the user presses on close dialog
    // which is eventually used in favorites-sk to decide if it wants to
    // re-fetch the favorites or not.
    return new Promise((resolve, reject) => {
      this.resolve = resolve;
      this.reject = reject;
    });
  }

  private filterName(e: Event): void {
    this.name = (e.target as HTMLInputElement).value;
    this._render();
  }

  private filterDescription(e: Event): void {
    this.description = (e.target as HTMLInputElement).value;
    this._render();
  }

  private filterUrl(e: Event): void {
    this.url = (e.target as HTMLInputElement).value;
    this._render();
  }

  private static template = (ele: FavoritesDialogSk) => html`
      <dialog id="favDialog">
        <h2>Favorite</h2>

        <button id="favCloseIcon" @click=${ele.dismiss}>
          <close-icon-sk></close-icon-sk>
        </button>

        <div id=spinContainer>
          <spinner-sk ?active=${ele.updatingFavorite}></spinner-sk>
        </div>

        <span class="label">
          <label for="name-${ele.uniqueId}">Name*</label>
        </span>
        <input
          id="name-${ele.uniqueId}"
          placeholder="Name"
          .value="${ele.name}"
          @input=${(e: Event) => ele.filterName(e)}>
        </input>
        <br/>

        <span class="label">
          <label for="desc-${ele.uniqueId}">Description</label>
        </span>
        <input
          id="desc-${ele.uniqueId}"
          placeholder="Description"
          .value="${ele.description}"
          @input=${(e: Event) => ele.filterDescription(e)}></input>
        <br/>

        <span class="label">
          <label for="url-${ele.uniqueId}">URL*</label>
        </span>
        <input
          id="url-${ele.uniqueId}"
          placeholder="URL"
          value="${ele.url}"
          @input=${(e: Event) => ele.filterUrl(e)}></input>
        <br/><br/>

        <div ?hidden="${!ele.updatingFavorite}">
          Working on it...
        </div>

        <div class="buttons">
          <button ?disabled="${ele.updatingFavorite}" @click=${ele.dismiss}>Cancel</button>
          <button ?disabled="${ele.updatingFavorite}" @click=${ele.confirm}>Save</button>
        </div>
      </dialog>`;
}

define('favorites-dialog-sk', FavoritesDialogSk);
