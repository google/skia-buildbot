import { html } from 'lit/html.js';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { define } from '../define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { icons } from './icons';

class IconsDemoSk extends ElementSk {
  private static template = (el: IconsDemoSk) => html`
    <h1>Icons demo</h1>

    <div class="search">
      <input
        type="text"
        placeholder="Filter icons by name"
        @input=${(e: Event) => el.onFilterInput(e)} />
      <button @click=${() => el.onClearClick()}>Clear</button>
    </div>

    ${el.getCategories().length === 0
      ? html`<p class="no-results">No icons match "${el.filter}".</p>`
      : el
          .getCategories()
          .map((category: string) =>
            IconsDemoSk.categoryTemplate(el, category)
          )}
  `;

  private static categoryTemplate = (el: IconsDemoSk, category: string) => html`
    <div class="category category-${category}">
      <h2>${category}</h2>
      <div class="icons">
        ${el
          .getIconsForCategory(category)
          .map((iconName: string) => IconsDemoSk.iconTemplate(iconName))}
      </div>
    </div>
  `;

  private static iconTemplate = (iconName: string) => html`
    <figure class="icon">
      ${unsafeHTML(
        `<${iconName}-icon-sk title="${iconName}-icon-sk"></${iconName}-icon-sk>`
      )}
      <figcaption>${iconName}</figcaption>
    </figure>
  `;

  private filter = '';

  constructor() {
    super(IconsDemoSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private getCategories(): string[] {
    return Array.from(icons.keys()).filter(
      (cat) => this.getIconsForCategory(cat).length > 0
    );
  }

  private getIconsForCategory(category: string): string[] {
    return icons.get(category)!.filter((icon) => icon.includes(this.filter));
  }

  private onFilterInput(e: Event) {
    const element = e.target as HTMLInputElement;
    this.filter = element.value;
    this._render();
  }

  private onClearClick() {
    this.querySelector<HTMLInputElement>('.search input')!.value = '';
    this.filter = '';
    this._render();
  }
}

define('icons-demo-sk', IconsDemoSk);
