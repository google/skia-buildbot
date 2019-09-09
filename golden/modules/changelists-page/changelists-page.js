import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { define } from 'elements-sk/define'
import { html } from 'lit-html'


const template = (ele) => html`
<div>hello world</div>
`;

define('changelists-page', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback()
    this._render();
  }

});