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
    fName: 'AbrilFatface-Regular',
    fStyle: 'Regular',
    fFamily: 'AbrilFatface',
  },
  {
    fName: 'AmaticSC-Bold',
    fStyle: 'Bold',
    fFamily: 'AmaticSC',
  },
  {
    fName: 'Anton-Regular',
    fStyle: 'Regular',
    fFamily: 'Anton',
  },
  {
    fName: 'Archivo-BoldItalic',
    fStyle: 'BoldItalic',
    fFamily: 'Archivo',
  },
  {
    fName: 'Archivo Narrow-Regular',
    fStyle: 'Regular',
    fFamily: 'Archivo Narrow',
  },
  {
    fName: 'Bahiana-Regular',
    fStyle: 'Regular',
    fFamily: 'Bahiana',
  },
  {
    fName: 'BarlowCondensed-Regular',
    fStyle: 'Regular',
    fFamily: 'BarlowCondensed',
  },
  {
    fName: 'BarlowCondensed-SemiBold',
    fStyle: 'SemiBold',
    fFamily: 'BarlowCondensed',
  },
  {
    fName: 'Boogaloo-Regular',
    fStyle: 'Regular',
    fFamily: 'Boogaloo',
  },
  {
    fName: 'CaveatBrush-Regular',
    fStyle: 'Regular',
    fFamily: 'CaveatBrush',
  },
  {
    fName: 'Caveat-Regular',
    fStyle: 'Regular',
    fFamily: 'Caveat',
  },
  {
    fName: 'Chango-Regular',
    fStyle: 'Regular',
    fFamily: 'Chango',
  },
  {
    fName: 'ChelaOne-Regular',
    fStyle: 'Regular',
    fFamily: 'ChelaOne',
  },
  {
    fName: 'Chewy-Regular',
    fStyle: 'Regular',
    fFamily: 'Chewy',
  },
  {
    fName: 'Comfortaa-Light',
    fStyle: 'Light',
    fFamily: 'Comfortaa',
  },
  {
    fName: 'Comfortaa-Regular',
    fStyle: 'Regular',
    fFamily: 'Comfortaa',
  },
  {
    fName: 'Corben-Bold',
    fStyle: 'Bold',
    fFamily: 'Corben',
  },
  {
    fName: 'Courgette-Regular',
    fStyle: 'Regular',
    fFamily: 'Courgette',
  },
  {
    fName: 'CoveredByYourGrace-Regular',
    fStyle: 'Regular',
    fFamily: 'CoveredByYourGrace',
  },
  {
    fName: 'Creepster-Regular',
    fStyle: 'Regular',
    fFamily: 'Creepster',
  },
  {
    fName: 'Damion-Regular',
    fStyle: 'Regular',
    fFamily: 'Damion',
  },
  {
    fName: 'DMSans-Regular',
    fStyle: 'Regular',
    fFamily: 'DMSans',
  },
  {
    fName: 'EBGaramond-Medium',
    fStyle: 'Medium',
    fFamily: 'EBGaramond',
  },
  {
    fName: 'EBGaramond-Regular',
    fStyle: 'Regular',
    fFamily: 'EBGaramond',
  },
  {
    fName: 'FredokaOne-Regular',
    fStyle: 'Regular',
    fFamily: 'FredokaOne',
  },
  {
    fName: 'GermaniaOne-Regular',
    fStyle: 'Regular',
    fFamily: 'GermaniaOne',
  },
  {
    fName: 'JollyLodger-Regular',
    fStyle: 'Regular',
    fFamily: 'JollyLodger',
  },
  {
    fName: 'Knewave-Regular',
    fStyle: 'Regular',
    fFamily: 'Knewave',
  },
  {
    fName: 'KronaOne-Regular',
    fStyle: 'Regular',
    fFamily: 'KronaOne',
  },
  {
    fName: 'Lexend-Regular',
    fStyle: 'Regular',
    fFamily: 'Lexend',
  },
  {
    fName: 'LifeSavers-ExtraBold',
    fStyle: 'ExtraBold',
    fFamily: 'LifeSavers',
  },
  {
    fName: 'Lobster-Regular',
    fStyle: 'Regular',
    fFamily: 'Lobster',
  },
  {
    fName: 'LondrinaSolid-Black',
    fStyle: 'Black',
    fFamily: 'LondrinaSolid',
  },
  {
    fName: 'Lora-Regular',
    fStyle: 'Regular',
    fFamily: 'Lora',
  },
  {
    fName: 'LuckiestGuy-Regular',
    fStyle: 'Regular',
    fFamily: 'LuckiestGuy',
  },
  {
    fName: 'Merriweather-Regular',
    fStyle: 'Regular',
    fFamily: 'Merriweather',
  },
  {
    fName: 'Metamorphous-Regular',
    fStyle: 'Regular',
    fFamily: 'Metamorphous',
  },
  {
    fName: 'Modak-Regular',
    fStyle: 'Regular',
    fFamily: 'Modak',
  },
  {
    fName: 'Molle-Regular',
    fStyle: 'Regular',
    fFamily: 'Molle',
  },
  {
    fName: 'Monoton-Regular',
    fStyle: 'Regular',
    fFamily: 'Monoton',
  },
  {
    fName: 'Montserrat-Black',
    fStyle: 'Black',
    fFamily: 'Montserrat',
  },
  {
    fName: 'Montserrat-Bold',
    fStyle: 'Bold',
    fFamily: 'Montserrat',
  },
  {
    fName: 'Montserrat-Regular',
    fStyle: 'Regular',
    fFamily: 'Montserrat',
  },
  {
    fName: 'Neonderthaw-Regular',
    fStyle: 'Regular',
    fFamily: 'Neonderthaw',
  },
  {
    fName: 'NewRocker-Regular',
    fStyle: 'Regular',
    fFamily: 'NewRocker',
  },
  {
    fName: 'Noto Sans-Regular',
    fStyle: 'Regular',
    fFamily: 'Noto Sans',
  },
  {
    fName: 'Noto Sans Mono-Regular',
    fStyle: 'Regular',
    fFamily: 'Noto Sans Mono',
  },
  {
    fName: 'Noto Serif-Regular',
    fStyle: 'Regular',
    fFamily: 'Noto Serif',
  },
  {
    fName: 'Overlock-BlackItalic',
    fStyle: 'BlackItalic',
    fFamily: 'Overlock',
  },
  {
    fName: 'Oswald-Regular',
    fStyle: 'Regular',
    fFamily: 'Oswald',
  },
  {
    fName: 'Oswald-Bold',
    fStyle: 'Bold',
    fFamily: 'Oswald',
  },
  {
    fName: 'Pacifico-Regular',
    fStyle: 'Regular',
    fFamily: 'Pacifico',
  },
  {
    fName: 'PermanentMarker-Regular',
    fStyle: 'Regular',
    fFamily: 'PermanentMarker',
  },
  {
    fName: 'PlayfairDisplay-Regular',
    fStyle: 'Regular',
    fFamily: 'PlayfairDisplay',
  },
  {
    fName: 'PlayfairDisplay-SemiBoldItalic',
    fStyle: 'SemiBoldItalic',
    fFamily: 'PlayfairDisplay',
  },
  {
    fName: 'Poppins-BlackItalic',
    fStyle: 'BlackItalic',
    fFamily: 'Poppins',
  },
  {
    fName: 'Ranchers-Regular',
    fStyle: 'Regular',
    fFamily: 'Ranchers',
  },
  {
    fName: 'Righteous-Regular',
    fStyle: 'Regular',
    fFamily: 'Righteous',
  },
  {
    fName: 'Roboto-Regular',
    fStyle: 'Regular',
    fFamily: 'Roboto',
  },
  {
    fName: 'Roboto Mono-Regular',
    fStyle: 'Regular',
    fFamily: 'Roboto Mono',
  },
  {
    fName: 'SairaCondensed-ExtraBold',
    fStyle: 'ExtraBold',
    fFamily: 'SairaCondensed',
  },
  {
    fName: 'Shrikhand-Regular',
    fStyle: 'Regular',
    fFamily: 'Shrikhand',
  },
  {
    fName: 'Slackey-Regular',
    fStyle: 'Regular',
    fFamily: 'Slackey',
  },
  {
    fName: 'Sniglet-ExtraBold',
    fStyle: 'ExtraBold',
    fFamily: 'Sniglet',
  },
  {
    fName: 'Spectral-BoldItalic',
    fStyle: 'BoldItalic',
    fFamily: 'Spectral',
  },
  {
    fName: 'Spectral-Regular',
    fStyle: 'Regular',
    fFamily: 'Spectral',
  },
  {
    fName: 'SpicyRice-Regular',
    fStyle: 'Regular',
    fFamily: 'SpicyRice',
  },
  {
    fName: 'Sriracha-Regular',
    fStyle: 'Regular',
    fFamily: 'Sriracha',
  },
  {
    fName: 'Syncopate-Bold',
    fStyle: 'Bold',
    fFamily: 'Syncopate',
  },
  {
    fName: 'TextMeOne-Regular',
    fStyle: 'Regular',
    fFamily: 'TextMeOne',
  },
  {
    fName: 'TitilliumWeb-Black',
    fStyle: 'Black',
    fFamily: 'TitilliumWeb',
  },
  {
    fName: 'Tomorrow-ExtraBoldItalic',
    fStyle: 'ExtraBoldItalic',
    fFamily: 'Tomorrow',
  },
  {
    fName: 'Warnes-Regular',
    fStyle: 'Regular',
    fFamily: 'Warnes',
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
