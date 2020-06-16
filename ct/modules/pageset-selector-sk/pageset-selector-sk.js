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

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/select-sk';
import '../../../infra-sk/modules/expandable-textarea-sk';

const template = (ele) => html`
<div class=pageset-list>
<select-sk>
  ${ele._pageSets.map((p) => (html`<div>${p.description}</div>`))}
</select-sk>
</div>
${ele.hasAttribute('disable-custom-webpages')
    ? ''
    : customWebpageFormTemplate(ele)}
`;

const customFormPlaceholder = `Eg: webpage1,webpage2,webpage3

commas in webpages should be URL encoded`;

const customWebpageFormTemplate = (ele) => html`
<expandable-textarea-sk minRows=5
  displaytext="Specify custom list of web pages"
  placeholder=${customFormPlaceholder}
  @click=${ele._updatePageSetHidden}>
</expandable-textarea-sk>
`;

define('pageset-selector-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._upgradeProperty('hideIfContains');
    this._upgradeProperty('selected');
    this._pageSets = this._pageSets || [];
    this._selected = this._selected || '';
    this._hideIfKeyContains = this._hideIfKeyContains || [];
  }

  connectedCallback() {
    super.connectedCallback();
    fetch('/_/page_sets/', { method: 'POST' })
      .then(jsonOrThrow)
      .then((json) => {
        this._pageSets = json;
        this._filterPageSets();
        this._render();
        this._selector.selection = 0;
      })
      .catch(errorMessage);
    this._render();
    this._selector = $$('select-sk', this);
  }

  /**
   * @prop {string} customPages - User Supplied custom webpage string.
   */
  get customPages() {
    const exTextarea = $$('expandable-textarea-sk', this);
    return exTextarea ? (exTextarea.value || '') : '';
  }

  /**
   * @prop {string} selected - Key of selected pageset.
   */
  get selected() {
    const index = this._selector.selection;
    return index >= 0 ? this._pageSets[index].key : '';
  }

  set selected(val) {
    this._selector.selection = this._pageSets.findIndex((p) => p.key === val);
  }

  /**
   * @prop {Array<string>} hideIfKeyContains - Entries containing these substrings
   * are removed from the pageSet listing.
   */
  get hideIfKeyContains() {
    return this._hideIfKeyContains;
  }

  set hideIfKeyContains(val) {
    this._hideIfKeyContains = val;
    this._filterPageSets();
    this._render();
  }

  _filterPageSets() {
    const blacklist = this._hideIfKeyContains;
    const psHasSubstring = (ps) => blacklist.some((s) => ps.key.includes(s));
    this._pageSets = this._pageSets.filter((ps) => !psHasSubstring(ps));
  }

  _updatePageSetHidden() {
    const exTextArea = $$('expandable-textarea-sk', this);
    const pageSetContainer = $$('.pageset-list', this);
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
});
