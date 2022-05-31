/**
 * @module modules/pageset-selector-sk
 * @description A custom element for selecting pagesets, and optionally
 * listing custom webpages.
 *
 * @attr {Boolean} disable-custom-webpages - When set, don't offer users
 * the option to input custom webpages.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { SelectSk } from 'elements-sk/select-sk/select-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/select-sk';
import '../../../infra-sk/modules/expandable-textarea-sk';

import {
  PageSet,
} from '../json';

export interface ExpandableTextArea extends HTMLInputElement {
  open: boolean;
}

export class PagesetSelectorSk extends ElementSk {
  private static customFormPlaceholder = `Eg: webpage1,webpage2,webpage3

  commas in webpages should be URL encoded`;

  private _pageSets: PageSet[] = [];

  private _unfilteredPageSets: PageSet[] = [];

  private _selector: SelectSk | null = null;

  private _hideKeys: string[] = [];

  constructor() {
    super(PagesetSelectorSk.template);

    this._upgradeProperty('hideIfContains');
    this._upgradeProperty('selected');
  }

  private static template = (ele: PagesetSelectorSk) => html`
<div class=pageset-list>
<select-sk>
  ${ele._pageSets.map((p) => (html`<div>${p.description}</div>`))}
</select-sk>
</div>
${ele.hasAttribute('disable-custom-webpages')
    ? ''
    : PagesetSelectorSk.customWebpageFormTemplate(ele)}
`;

  private static customWebpageFormTemplate = (ele: PagesetSelectorSk) => html`
<expandable-textarea-sk minRows=5
  displaytext="Specify custom list of web pages"
  placeholder=${PagesetSelectorSk.customFormPlaceholder}
  @click=${ele._updatePageSetHidden}>
</expandable-textarea-sk>
`;

  connectedCallback(): void {
    super.connectedCallback();
    fetch('/_/page_sets/', { method: 'POST' })
      .then(jsonOrThrow)
      .then((json: PageSet[]) => {
        this._unfilteredPageSets = json;
        this._filterPageSets();
        this._render();
        // Always start with the default 10k.
        this.selected = '10k';
      })
      .catch(errorMessage);
    this._render();
    this._selector = $$('select-sk', this);
  }

  /**
   * @prop {string} customPages - User Supplied custom webpage string.
   */
  get customPages(): string {
    const exTextarea = $$('expandable-textarea-sk', this) as HTMLInputElement;
    return exTextarea ? (exTextarea.value || '') : '';
  }

  set customPages(val: string) {
    const exTextarea = $$('expandable-textarea-sk', this) as ExpandableTextArea;
    exTextarea.value = val;
  }

  /**
   * @prop {string} selected - Key of selected pageset.
   */
  get selected(): string {
    const index = this._selector!.selection as number;
    return index >= 0 ? this._pageSets[index].key : '';
  }

  set selected(val: string) {
    this._selector!.selection = this._pageSets.findIndex((p) => p.key === val);
  }

  /**
   * @prop {Array<string>} hideKeys - Entries containing these keys
   * are removed from the pageSet listing.
   */
  get hideKeys(): string[] {
    return this._hideKeys;
  }

  set hideKeys(val: string[]) {
    this._hideKeys = val;
    this._filterPageSets();
    this._render();
  }

  /**
   * Expands the text area if it is collapsed.
   */
  expandTextArea(): void {
    const exTextarea = $$('expandable-textarea-sk', this) as ExpandableTextArea;
    if (!exTextarea.open) {
      ($$('button', exTextarea) as HTMLElement).click();
    }
  }

  _filterPageSets(): void {
    this._pageSets = this._unfilteredPageSets
      .filter((ps) => !this._hideKeys.includes(ps.key));
  }

  _updatePageSetHidden(): void {
    const exTextArea = $$('expandable-textarea-sk', this) as ExpandableTextArea;
    const pageSetContainer = $$('.pageset-list', this) as HTMLElement;
    if (exTextArea.open === pageSetContainer.hidden) {
    // This click wasn't toggling the expandable textarea.
      return;
    }
    pageSetContainer.hidden = exTextArea.open;
    if (!exTextArea.open) {
    // We assume if someone closes the panel they don't want any custom pages.
      exTextArea.value = '';
    }
    this._render();
  }
}

define('pageset-selector-sk', PagesetSelectorSk);
