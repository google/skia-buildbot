/**
 * @module modules/suggest-input-sk
 * @description A custom element that implements regex and substring match
 * suggestions. These are selectable via click or up/down/enter.
 *
 * @attr {Boolean} accept-custom-value - Whether users can enter values not listed
 * in this.options.
 *
 * @event value-changed - Any time the user selected or inputted value is
 * committed. Event is of the form { value: <newValue> }
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const DOWN_ARROW = '40';
const UP_ARROW = '38';
const ENTER = '13';

export class SuggestInputSk extends ElementSk {
  private _options: string[] = [];

  private _suggestions: string[] = [];

  private _suggestionSelected: number = -1;

  private _label: string = '';

  constructor() {
    super(SuggestInputSk.template);

    this._upgradeProperty('options');
    this._upgradeProperty('acceptCustomValue');
    this._upgradeProperty('label');
  }

  // TODO(westont): We should probably use input-sk here.
  private static template = (ele: SuggestInputSk) => html`
<div class=suggest-input-container>
<input class=suggest-input autocomplete=off required
  @focus=${ele._refresh}
  @input=${ele._refresh}
  @keyup=${ele._keyup}
  @blur=${ele._blur}>
</input>
<label class="suggest-label">${ele.label}</label>
<div class=suggest-underline-container>
  <div class=suggest-underline></div>
  <div class=suggest-underline-background ></div>
</div>
<div class=suggest-list
  ?hidden=${!(ele._suggestions && ele._suggestions.length > 0)}
  @click=${ele._suggestionClick}>
  <ul>
  ${ele._suggestions.map((s, i) => (ele._suggestionSelected === i
    ? SuggestInputSk.selectedOptionTemplate(s) : SuggestInputSk.optionTemplate(s)))}
  </ul>
</div>
</div>
`;

  // tabindex so the fields populate FocusEvent.relatedTarget on blur.
  private static optionTemplate = (option: string) => html`
<li tabindex=-1 class=suggestion>${option}</li>
`;

  private static selectedOptionTemplate = (option: string) => html`
<li tabindex=-1 class="suggestion selected">${option}</li>
`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop {string} value - Content of the input element from typing,
   * selection, etc.
   */
  get value(): string {
  // We back our value with input.value directly, to avoid issues with the
  // input value changing without changing our value property, causing
  // element re-rendering to be skipped.
    return ($$('input', this) as HTMLInputElement).value;
  }

  set value(v: string) {
    ($$('input', this) as HTMLInputElement).value = v;
  }

  /**
   * @prop {Array<string>} options - Values for suggestion list.
   */
  get options(): string[] {
    return this._options;
  }

  set options(o: string[]) {
    this._options = o;
  }

  /**
   * @prop {Boolean} acceptCustomValue - Mirrors the
   * 'accept-custom-value' attribute.
   */
  get acceptCustomValue(): boolean {
    return this.hasAttribute('accept-custom-value');
  }

  set acceptCustomValue(val: boolean) {
    if (val) {
      this.setAttribute('accept-custom-value', '');
    } else {
      this.removeAttribute('accept-custom-value');
    }
  }

  /**
   * @prop string label - Label to display to guide user input.
   */
  get label(): string {
    return this._label;
  }

  set label(o: string) {
    this._label = o;
  }

  _blur(e: MouseEvent): void {
  // Ignore if this blur is preceding _suggestionClick.
    const blurredElem = e.relatedTarget as HTMLElement;
    if (blurredElem && blurredElem.classList.contains('suggestion')) {
      return;
    }
    this._commit();
  }

  _commit(): void {
    if (this._suggestionSelected > -1) {
      this.value = this._suggestions[this._suggestionSelected];
    } else if (!this._options.includes(this.value) && !this.acceptCustomValue) {
      this.value = '';
    }
    this._suggestions = [];
    this._suggestionSelected = -1;
    this._render();
    this.dispatchEvent(new CustomEvent('value-changed',
      { bubbles: true, detail: { value: this.value } }));
  }

  _keyup(e: KeyboardEvent): void {
  // Allow the user to scroll through suggestions using arrow keys.
    const len = this._suggestions.length;
    const key = e.key || e.code;
    if ((key === 'ArrowDown' || key === DOWN_ARROW) && len > 0) {
      this._suggestionSelected = (this._suggestionSelected + 1) % len;
      this._render();
    } else if ((key === 'ArrowUp' || key === UP_ARROW) && len > 0) {
      this._suggestionSelected = (this._suggestionSelected + len - 1) % len;
      this._render();
    } else if (key === 'Enter' || key === ENTER) {
    // This also commits the current selection (if present) or custom
    // value (if allowed).
      ($$('input', this) as HTMLInputElement).dispatchEvent(
        new Event('blur', { bubbles: true, cancelable: true }),
      );
    }
  }

  _refresh(): void {
    const v = this.value;
    let re: {test: (str: string)=> boolean; };
    try {
      re = new RegExp(v, 'i'); // case-insensitive.
    } catch (err) {
    // If the user enters an invalid expression, just use substring
    // match.
      re = {
        test: function(str: string) {
          return str.indexOf(v) !== -1;
        },
      };
    }
    this._suggestions = this._options.filter((s) => re.test(s));
    this._suggestionSelected = -1;
    this._render();
  }

  _suggestionClick(e: Event): void {
    const item = e.target as HTMLElement;
    if (item.tagName !== 'LI') {
      return;
    }
    const index = Array.from(item.parentNode!.children).indexOf(item);
    this._suggestionSelected = index;
    this._commit();
  }
}

define('suggest-input-sk', SuggestInputSk);
