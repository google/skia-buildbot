/**
 * @module modules/list-page-sk
 * @description <h2><code>list-page-sk</code></h2>
 *
 * A page comprising a filterable list of things, presented as a multi-column
 * table. Fill out the abstract properties to parametrize.
 *
 * @attr waiting - If present then display the waiting cursor.
 */
import { html, TemplateResult } from 'lit-html';

import { $$ } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/error-toast-sk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { FilterArray } from '../filter-array';
import '../auto-refresh-sk';
import '../device-editor-sk';
import '../note-editor-sk';
import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

export abstract class ListPageSk<ItemType> extends ElementSk {
  protected _filterer: FilterArray<ItemType> = new FilterArray();

  /**
   * The URL path from which to fetch the JSON representation of the latest
   * list items
   */
  protected abstract _fetchPath: string;

  /** Return all the <th>s for the table (with no <tr> around them). */
  abstract tableHeaders(): TemplateResult;

  /** Return a <tr> displaying a single item. */
  abstract tableRow(item: ItemType): TemplateResult;

  protected _template = (ele: ListPageSk<ItemType>): TemplateResult => html`
    <header>
      <auto-refresh-sk @refresh-page=${ele.update}></auto-refresh-sk>
      <span id=header-rhs>
        <input id=filter-input type="text" placeholder="Filter">
        <theme-chooser-sk title="Toggle between light and dark mode."></theme-chooser-sk>
      </span>
    </header>
    <main>
      <table>
        <thead>
          <tr>
            ${ele.tableHeaders()}
          </tr>
        </thead>
        <tbody>
          ${ele.tableRows()}
        </tbody>
      </table>
    </main>
    <!-- TODO(kjlubick) These should not go here, but only in the subclass -->
    <note-editor-sk></note-editor-sk>
    <device-editor-sk></device-editor-sk>
    <error-toast-sk></error-toast-sk>
  `;

  constructor() {
    super();
    this.classList.add('defaultListPageSkStyling');
  }

  /**
   * Return <tr>s for the items which match the active filter.
   *
   * The default implementation delegates to tableRow(), passing it each item
   * in turn.
   */
  tableRows(): TemplateResult[] {
    return this._filterer.matchingValues().map((item) => this.tableRow(item));
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
    const filterInput = $$<HTMLInputElement>('#filter-input', this)!;
    this._filterer.connect(filterInput, () => this._render());
    await this.update();
  }

  /**
   * Fetch the latest list from the server, and update the page to reflect it.
   */
  async update(changeCursor = false): Promise<void> {
    if (changeCursor) {
      this.setAttribute('waiting', '');
    }

    try {
      const resp = await fetch(this._fetchPath);
      const json = await jsonOrThrow(resp);
      if (changeCursor) {
        this.removeAttribute('waiting');
      }
      this._filterer.updateArray(json);
      this._render();
    } catch (error) {
      this.onError(error);
    }
  }

  onError(msg: { message: string; } | string): void {
    this.removeAttribute('waiting');
    errorMessage(msg);
  }
}
