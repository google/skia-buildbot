/**
 * @module modules/corpus-selector-sk
 * @description <h2><code>corpus-selector-sk</code></h2>
 *
 * Lists the available corpora and lets the user select a corpus.
 *
 * @evt corpus_selected - Sent when the user selects a different corpus. Field
 *      event.detail.corpus will contain the selected corpus.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (ele) => {
  if (!ele.corpora.length) {
    return html`<p>Loading corpora details...</p>`;
  }
  return html`<ul>${ele.corpora.map((corpus) => corpusItem(ele, corpus))}</ul>`;
};

const corpusItem = (ele, corpus) => html`
<li class=${ele.selectedCorpus === corpus.name ? 'selected' : ''}
    title="${ele.corpusRendererFn(corpus)}"
    @click=${() => ele._handleCorpusClick(corpus.name)}>
  ${ele.corpusRendererFn(corpus)}
</li>`;

define('corpus-selector-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._corpora = [];
    // Default to just showing the corpus name.
    this._corpusRendererFn = (corpus) => corpus.name;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop corpora {Array<Object>} An array of objects that have at least the name field. There
   *    can be additional information that is made available through the corpusRendererFn.
   */
  get corpora() { return this._corpora; }

  set corpora(arr) {
    this._corpora = arr;
    this._render();
  }

  /**
   * @prop corpusRendererFn {function} A function that takes a corpus object and
   *     returns the text to be displayed on the corpus selector widget.
   */
  get corpusRendererFn() { return this._corpusRendererFn; }

  set corpusRendererFn(fn) {
    this._corpusRendererFn = fn;
    this._render();
  }

  /** @prop selectedCorpus {string} The selected corpus name. */
  get selectedCorpus() { return this._selectedCorpus; }

  set selectedCorpus(corpus) {
    this._selectedCorpus = corpus;
    this._render();
  }

  _handleCorpusClick(corpus) {
    if (this.selectedCorpus !== corpus) {
      this.selectedCorpus = corpus;
      this._sendCorpusSelected();
    }
  }

  _sendCorpusSelected() {
    this.dispatchEvent(
      new CustomEvent('corpus_selected', {
        detail: {
          corpus: this.selectedCorpus,
        },
        bubbles: true,
      }),
    );
  }
});
