/**
 * @module modules/corpus-selector-sk
 * @description <h2><code>corpus-selector-sk</code></h2>
 *
 * An element that allows the user to select a corpus from a list of available corpora.
 *
 * Events:
 *
 *   corpus-selected: Emitted when the user selects a corpus. The event details contains the
 *                    selected corpus.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

/**
 * Takes a corpus object and returns a string used to represent the corpus on the CorpusSelectorSk
 * element's UI.
 */
export type CorpusRendererFn<T extends Object> = (corpus: T)=> string;

/**
 * An element that allows the user to select a corpus from a list of available corpora.
 *
 * Corpora are objects of the generic type T, and are transformed to text to be displayed in the UI
 * using T#toString(). This behavior can be overridden by providing a custom CorpusRendererFn<T>
 * via the customRendererFn property.
 */
export class CorpusSelectorSk<T extends Object> extends ElementSk {
  private static template =
    <T extends Object>(el: CorpusSelectorSk<T>) => (el._corpora.length
      ? html`
            <ul>${el._corpora.map((corpus) => CorpusSelectorSk.corpusTemplate(el, corpus))}</ul>`
      : html`<p>Loading corpora details...</p>`);

  private static corpusTemplate =
    <T extends Object>(el: CorpusSelectorSk<T>, corpus: T) => html`
        <li class=${el._selectedCorpus === corpus ? 'selected' : ''}
            title="${el._corpusRendererFn(corpus)}"
            @click=${() => el._handleCorpusClick(corpus)}>
          ${el._corpusRendererFn(corpus)}
        </li>`;

  private _corpora: T[] = [];

  // Default to the corpus object's toString() method.
  private _corpusRendererFn: CorpusRendererFn<T> = (corpus) => corpus.toString();

  private _selectedCorpus: T | null = null;

  constructor() {
    super(CorpusSelectorSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** The corpora available for the user to select from. */
  get corpora() { return this._corpora; }

  set corpora(value) {
    this._corpora = value;
    this._render();
  }

  /** A function that takes a corpus and returns the text to display on the UI. */
  get corpusRendererFn() { return this._corpusRendererFn; }

  set corpusRendererFn(fn) {
    this._corpusRendererFn = fn;
    this._render();
  }

  /** The currently selected corpus. */
  get selectedCorpus() { return this._selectedCorpus; }

  set selectedCorpus(corpus) {
    this._selectedCorpus = corpus;
    this._render();
  }

  private _handleCorpusClick(corpus: T) {
    if (this.selectedCorpus !== corpus) {
      this.selectedCorpus = corpus;
      this._sendCorpusSelected();
    }
  }

  private _sendCorpusSelected() {
    this.dispatchEvent(
      new CustomEvent<T>('corpus-selected', {
        detail: this._selectedCorpus!,
        bubbles: true,
      }),
    );
  }
}

define('corpus-selector-sk', CorpusSelectorSk);
