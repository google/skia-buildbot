/**
 * @module modules/list-page-sk
 * @description <h2><code>list-page-sk</code></h2>
 *
 * A page comprising a filterable table of things pulled from a REST endpoint.
 * Fill out the abstract properties to parametrize. The table data doesn't
 * fetch and draw until update() is called.
 *
 * @attr waiting - If present then display the waiting cursor.
 */
import { html, TemplateResult } from 'lit-html';

import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { FilterArray } from '../filter-array';
import '../auto-refresh-sk';
import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

export enum WaitCursor {
  DO_NOT_SHOW,
  SHOW,
}

export abstract class ListPageSk<ItemType> extends ElementSk {
  protected filterer: FilterArray<ItemType> = new FilterArray();

  /**
   * The URL path from which to fetch the JSON representation of the latest
   * list items
   */
  protected abstract fetchPath: string;

  /** Return all the <th>s for the table (with no <tr> around them). */
  abstract tableHeaders(): TemplateResult;

  /** Return a <tr> displaying a single item. */
  abstract tableRow(item: ItemType): TemplateResult;

  protected _template = (ele: ListPageSk<ItemType>): TemplateResult => html`
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
    ${ele.moreTemplate()}
  `;

  /**
   * Return additional markup to be included at the end of the element's
   * template. This is useful for UI dynamically shown by code in the
   * concretizing subclass.
   */
  protected moreTemplate(): TemplateResult {
    return html``;
  }

  constructor() {
    super();
    this.classList.add('defaultListPageSkStyling'); // TODO(erikrose): Rename to "liveTableSk" or something.
  }

  /**
   * Return <tr>s for the items which match the active filter.
   *
   * The default implementation delegates to tableRow(), passing it each item
   * in turn.
   */
  tableRows(): TemplateResult[] {
    return this.filterer.matchingValues().map((item) => this.tableRow(item));
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
  }

  /**
   * Show and hide rows to reflect a change in the filtration string.
   */
  filterChanged(value: string) {
    this.filterer.filterChanged(value);
    this._render();
  }

  /**
   * Fetch the latest list from the server, and update the page to reflect it.
   *
   * @param showWaitCursor Whether the mouse pointer should be changed to a
   *   spinner while we wait for the fetch
   */
  async update(waitCursorPolicy: WaitCursor = WaitCursor.DO_NOT_SHOW): Promise<void> {
    if (waitCursorPolicy === WaitCursor.SHOW) {
      this.setAttribute('waiting', '');
    }

    try {
      const resp = await fetch(this.fetchPath);
      const json = await jsonOrThrow(resp);
      if (waitCursorPolicy === WaitCursor.SHOW) {
        this.removeAttribute('waiting');
      }
      this.filterer.updateArray(json);
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
