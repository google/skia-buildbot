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
import { define } from '../../../elements-sk/modules/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../skottie-dropdown-sk';
import { DropdownSelectEvent } from '../skottie-dropdown-sk/skottie-dropdown-sk';

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

const availableFonts: FontType[] = [
  {
    fName: 'Righteous-Regular',
    fStyle: 'Righteous',
    fFamily: 'Regular',
  },
  {
    fName: 'BarlowCondensed-Regular',
    fStyle: 'Regular',
    fFamily: 'BarlowCondensed',
  },
  {
    fName: 'Anton-Regular',
    fStyle: 'Regular',
    fFamily: 'Anton',
  },
  {
    fName: 'DMSans-Regular',
    fStyle: 'Regular',
    fFamily: 'DMSans',
  },
  {
    fName: 'KronaOne-Regular',
    fStyle: 'Regular',
    fFamily: 'KronaOne',
  },
  {
    fName: 'BarlowCondensed-SemiBold',
    fStyle: 'SemiBold',
    fFamily: 'BarlowCondensed',
  },
  {
    fName: 'Archivo-BoldItalic',
    fStyle: 'BoldItalic',
    fFamily: 'Archivo',
  },
  {
    fName: 'Montserrat-Bold',
    fStyle: 'Bold',
    fFamily: 'Montserrat',
  },
  {
    fName: 'Syncopate-Bold',
    fStyle: 'Bold',
    fFamily: 'Syncopate',
  },
  {
    fName: 'SairaCondensed-ExtraBold',
    fStyle: 'ExtraBold',
    fFamily: 'SairaCondensed',
  },
  {
    fName: 'LuckiestGuy-Regular',
    fStyle: 'Regular',
    fFamily: 'LuckiestGuy',
  },
  {
    fName: 'Tomorrow-ExtraBoldItalic',
    fStyle: 'ExtraBoldItalic',
    fFamily: 'Tomorrow',
  },
  {
    fName: 'LondrinaSolid-Black',
    fStyle: 'Black',
    fFamily: 'LondrinaSolid',
  },
  {
    fName: 'Montserrat-Black',
    fStyle: 'Black',
    fFamily: 'Montserrat',
  },
  {
    fName: 'TitilliumWeb-Black',
    fStyle: 'Black',
    fFamily: 'TitilliumWeb',
  },
  {
    fName: 'Poppins-BlackItalic',
    fStyle: 'BlackItalic',
    fFamily: 'Poppins',
  },
  {
    fName: 'Comfortaa-Light',
    fStyle: 'Light',
    fFamily: 'Comfortaa',
  },
  {
    fName: 'Boogaloo-Regular',
    fStyle: 'Regular',
    fFamily: 'Boogaloo',
  },
  {
    fName: 'Chewy-Regular',
    fStyle: 'Regular',
    fFamily: 'Chewy',
  },
  {
    fName: 'Overlock-BlackItalic',
    fStyle: 'BlackItalic',
    fFamily: 'Overlock',
  },
  {
    fName: 'FredokaOne-Regular',
    fStyle: 'Regular',
    fFamily: 'FredokaOne',
  },
  {
    fName: 'Shrikhand-Regular',
    fStyle: 'Regular',
    fFamily: 'Shrikhand',
  },
  {
    fName: 'SpicyRice-Regular',
    fStyle: 'Regular',
    fFamily: 'SpicyRice',
  },
  {
    fName: 'Modak-Regular',
    fStyle: 'Regular',
    fFamily: 'Modak',
  },
  {
    fName: 'Chango-Regular',
    fStyle: 'Regular',
    fFamily: 'Chango',
  },
  {
    fName: 'Sniglet-ExtraBold',
    fStyle: 'ExtraBold',
    fFamily: 'Sniglet',
  },
  {
    fName: 'AmaticSC-Bold',
    fStyle: 'Bold',
    fFamily: 'AmaticSC',
  },
  {
    fName: 'CaveatBrush-Regular',
    fStyle: 'Regular',
    fFamily: 'CaveatBrush',
  },
  {
    fName: 'CoveredByYourGrace-Regular',
    fStyle: 'Regular',
    fFamily: 'CoveredByYourGrace',
  },
  {
    fName: 'Knewave-Regular',
    fStyle: 'Regular',
    fFamily: 'Knewave',
  },
  {
    fName: 'PermanentMarker-Regular',
    fStyle: 'Regular',
    fFamily: 'PermanentMarker',
  },
  {
    fName: 'Damion-Regular',
    fStyle: 'Regular',
    fFamily: 'Damion',
  },
  {
    fName: 'Neonderthaw-Regular',
    fStyle: 'Regular',
    fFamily: 'Neonderthaw',
  },
  {
    fName: 'Pacifico-Regular',
    fStyle: 'Regular',
    fFamily: 'Pacifico',
  },
  {
    fName: 'Lobster-Regular',
    fStyle: 'Regular',
    fFamily: 'Lobster',
  },
  {
    fName: 'Molle-Regular',
    fStyle: 'Regular',
    fFamily: 'Molle',
  },
  {
    fName: 'Bahiana-Regular',
    fStyle: 'Regular',
    fFamily: 'Bahiana',
  },
  {
    fName: 'JollyLodger-Regular',
    fStyle: 'Regular',
    fFamily: 'JollyLodger',
  },
  {
    fName: 'LifeSavers-ExtraBold',
    fStyle: 'ExtraBold',
    fFamily: 'LifeSavers',
  },
  {
    fName: 'Warnes-Regular',
    fStyle: 'Regular',
    fFamily: 'Warnes',
  },
  {
    fName: 'Ranchers-Regular',
    fStyle: 'Regular',
    fFamily: 'Ranchers',
  },
  {
    fName: 'Creepster-Regular',
    fStyle: 'Regular',
    fFamily: 'Creepster',
  },
  {
    fName: 'Slackey-Regular',
    fStyle: 'Regular',
    fFamily: 'Slackey',
  },
  {
    fName: 'Monoton-Regular',
    fStyle: 'Regular',
    fFamily: 'Monoton',
  },
  {
    fName: 'NewRocker-Regular',
    fStyle: 'Regular',
    fFamily: 'NewRocker',
  },
  {
    fName: 'ChelaOne-Regular',
    fStyle: 'Regular',
    fFamily: 'ChelaOne',
  },
  {
    fName: 'GermaniaOne-Regular',
    fStyle: 'Regular',
    fFamily: 'GermaniaOne',
  },
  {
    fName: 'Metamorphous-Regular',
    fStyle: 'Regular',
    fFamily: 'Metamorphous',
  },
  {
    fName: 'Spectral-BoldItalic',
    fStyle: 'BoldItalic',
    fFamily: 'Spectral',
  },
  {
    fName: 'Corben-Bold',
    fStyle: 'Bold',
    fFamily: 'Corben',
  },
  {
    fName: 'EBGaramond-Medium',
    fStyle: 'Medium',
    fFamily: 'EBGaramond',
  },
  {
    fName: 'PlayfairDisplay-SemiBoldItalic',
    fStyle: 'SemiBoldItalic',
    fFamily: 'PlayfairDisplay',
  },
  {
    fName: 'Merriweather-Regular',
    fStyle: 'Regular',
    fFamily: 'Merriweather',
  },
  {
    fName: 'AbrilFatface-Regular',
    fStyle: 'Regular',
    fFamily: 'AbrilFatface',
  },
  {
    fName: 'TextMeOne-Regular',
    fStyle: 'Regular',
    fFamily: 'TextMeOne',
  },
];

availableFonts.sort(function (a, b) {
  const textA = a.fName.toUpperCase();
  const textB = b.fName.toUpperCase();
  return textA < textB ? -1 : textA > textB ? 1 : 0;
});

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
