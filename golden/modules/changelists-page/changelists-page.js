import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'


const template = (ele) => html`
<div>hello world</div>
<div>${"ele is" + ele}<div>
`;

define('changelists-page', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    this.render();
  }

  render() {
    render(template(this), this, {eventContext: this})
  }

});