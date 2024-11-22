/**
 * @module skottie-font-selector-sk
 * @description <h2><code>skottie-font-selector-sk</code></h2>
 *
 * <p>
 *   A font selector to modify text layers from a skottie animation.
 *   The list of fonts (availableFonts) has been selected
 *   from a larger set of fonts available in a mirror of google web fonts.
 *   Refer here for more information:
 *   https://skia.googlesource.com/buildbot/+/refs/heads/main/skottie/modules/skottie-sk/skottie-sk.ts#1060
 * </p>
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../skottie-dropdown-sk';
import { DropdownSelectEvent } from '../skottie-dropdown-sk/skottie-dropdown-sk';
import availableFonts from '../helpers/availableFonts';

export type FontType = {
  fName: string;
  fStyle: string;
  fFamily: string;
};

type OptionType = {
  value: string;
  id: string;
  selected?: boolean;
};

export interface SkottieFontEventDetail {
  font: FontType;
}

export class SkottieFontSelectorSk extends ElementSk {
  private _fontName: string = '';

  private static template = (
    ele: SkottieFontSelectorSk
  ): TemplateResult => html`
    <div class="wrapper">
      <skottie-dropdown-sk
        id="view-exporter"
        .name="dropdown-exporter"
        .options=${ele.buildFontOptions()}
        @select=${ele.fontTypeSelectHandler}
        border
        full>
      </skottie-dropdown-sk>
    </div>
  `;

  constructor() {
    super(SkottieFontSelectorSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  private buildFontOptions(): OptionType[] {
    const fontOptions: OptionType[] = availableFonts.map((font) => {
      const isSelected = this._fontName === font.fName;
      return { id: font.fName, value: font.fName, selected: isSelected };
    });
    fontOptions.unshift({
      id: '',
      value: 'Select Font',
    });
    return fontOptions;
  }

  private fontTypeSelectHandler(ev: CustomEvent<DropdownSelectEvent>): void {
    // This event handler replaces the animation in place
    // instead of creating a copy of the lottie animation.
    // If there is a reason why it should create a copy, this can be updated
    if (ev.detail.value) {
      const newFontData = availableFonts.find(
        (font) => font.fName === ev.detail.value
      );
      if (newFontData) {
        this.dispatchEvent(
          new CustomEvent<SkottieFontEventDetail>('select-font', {
            detail: {
              font: newFontData,
            },
            bubbles: true,
          })
        );
      }
    }
  }

  set fontName(value: string) {
    this._fontName = value;
    this._render();
  }
}

define('skottie-font-selector-sk', SkottieFontSelectorSk);
