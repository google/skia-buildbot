/**
 * @module modules/corpus-selector-sk
 * @description <h2><code>corpus-selector-sk</code></h2>
 *
 * Lists the available corpora and lets the user select a corpus. Obtains the
 * available corpora from /json/trstatus.
 *
 * @attr update-freq-seconds {int} how often to ping the server for updates.
 *
 * @evt corpus-selected - Sent when the user selects a different corpus. Field
 *      event.detail.corpus will contain the selected corpus.
 */

import { define } from 'elements-sk/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { html } from 'lit-html';
import { classMap } from 'lit-html/directives/class-map.js';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

const template = (el) => html`
${!el.corpora ? html`<p>Loading corpora details...</p>` : html`
  <ul>
    ${el.corpora.map((corpus) => html`
      <li class=${classMap({selected: el.selectedCorpus === corpus.name})}
          title="${el.corpusRendererFn(corpus)}"
          @click=${() => el._handleCorpusClick(corpus.name)}>
        ${el.corpusRendererFn(corpus)}
      </li>
    `)}
  </ul>
`}
`;

define('corpus-selector-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._corpusRendererFn = (corpus) => corpus.name;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render(); // Render loading indicator.
    this._fetch();
    if (this._updateFreqSeconds > 0) {
      this._interval =
          setInterval(() => this._fetch(), this._updateFreqSeconds * 1000);
    }
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    if (this._interval) {
      clearInterval(this._interval);
      this._interval = null;
    }
  }

  _fetch() {
    // Force only one fetch at a time. Abort any outstanding requests. Useful if
    // a request takes longer than the update frequency.
    if (this._fetchController) {
      this._fetchController.abort();
    }
    this._fetchController = new AbortController();

    fetch('/json/trstatus', {
      method: 'GET',
      signal: this._fetchController.signal
    })
        .then(jsonOrThrow)
        .then((json) => {
          this.corpora = json.corpStatus;
          this._render();
          this._sendLoaded();
        })
        .catch((e) => {
          this._sendError(e);
        });
  }

  get _updateFreqSeconds() {
    return +this.getAttribute('update-freq-seconds');
  }

  /**
   * @prop corpusRendererFn {function} A function that takes a corpus and
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

  // Intended to be used only from tests.
  _sendLoaded() {
    this.dispatchEvent(
        new CustomEvent('corpus-selector-sk-loaded', {bubbles: true}));
  }

  _sendCorpusSelected() {
    this.dispatchEvent(
        new CustomEvent('corpus-selected', {
          detail: {
            corpus: this.selectedCorpus
          }, bubbles: true,
        }));
  }

  _sendError(e) {
    this.dispatchEvent(new CustomEvent('fetch-error', {
      detail: {
        error: e,
        loading: 'corpus selector',
      }, bubbles: true
    }));
  }
});
