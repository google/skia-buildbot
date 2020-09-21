/**
 * @module modules/triage2-sk
 * @description <h2><code>triage2-sk</code></h2>
 *  A custom element that allows toggling between the three
 *  Status states of triage: positive, negative, and untriaged.
 *
 * @evt change - The value of e.detail is the new triage Status value,
 *    for example "positive", or "negative".
 *
 * @attr value - The state of triage, either "positive", "negative", or
 *    "untriaged".
 *
 * @example
 *   <triage2-sk value=positive></triage2-sk>
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/styles/buttons';
import { Status } from '../json';

// TODO(jcgregorio) Maybe go2ts could emit isFoo guard functions?
function isStatus(value: string): value is Status {
  const allowed = ['positive', 'negative', 'untriaged', ''];
  return allowed.indexOf(value) !== -1;
}

export class TriageSk extends ElementSk {
  constructor() {
    super(TriageSk.template);
  }

  private static _match = (a: Status, b: Status) => a === b;

  private static template = (ele: TriageSk) => html`
    <button
      class="positive"
      @click=${() => (ele.value = 'positive')}
      ?selected=${TriageSk._match(ele.value, 'positive')}
    >
      <check-circle-icon-sk title="Positive"></check-circle-icon-sk>
    </button>
    <button
      class="negative"
      @click=${() => (ele.value = 'negative')}
      ?selected=${TriageSk._match(ele.value, 'negative')}
    >
      <cancel-icon-sk title="Negative"></cancel-icon-sk>
    </button>
    <button
      class="untriaged"
      @click=${() => (ele.value = 'untriaged')}
      ?selected=${TriageSk._match(ele.value, 'untriaged')}
    >
      <help-icon-sk title="Untriaged"></help-icon-sk>
    </button>
  `;


  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('value');
    if (!this.value) {
      this.value = 'untriaged';
    }
    this._render();
  }

  static get observedAttributes(): string[] {
    return ['value'];
  }

  /** @prop value - The status, such as 'positive', 'negative', or 'untriaged'. */
  get value(): Status {
    const v = this.getAttribute('value') || '';
    if (isStatus(v)) {
      return v;
    }
    return 'untriaged';
  }

  set value(val: Status) {
    this.setAttribute('value', val);
  }

  attributeChangedCallback(_name: string, oldValue: string, newValue: string): void {
    if (oldValue !== newValue) {
      this._render();
      this.dispatchEvent(
        new CustomEvent<Status>('change', {
          detail: newValue as Status,
          bubbles: true,
        }),
      );
    }
  }
}

define('triage2-sk', TriageSk);
