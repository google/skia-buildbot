import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { define } from 'elements-sk/define'
import { html } from 'lit-html'

import 'elements-sk/spinner-sk'

const template = (el) => {
  if (el._hidden) {
    return html``;
  }
  return html`
  <div class="container">
    <spinner-sk active></spinner-sk>
    <span>${el._text}</span>
  </div>
`;
};

define('activity-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._hidden = true;
    this._text = "";
  }

  startSpinner(text) {
    this._hidden = false;
    this._text = text;
    this._render();
  }

  stopSpinner() {
    this._hidden = true;
    this._text = "";
    this._render();
  }

  get isSpinning() { return !this._hidden };
});