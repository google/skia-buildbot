// This module processes a text string to support multi code point characters
// such as regional flags, emojis, and other characters that are represented
// with more than one code point.
// It exposes a factory that takes a string as input and returns an iterable object
// that outputs every individual final character correctly formatted.

let combinedCharacters: number[] = [];
// Hindi characters
combinedCharacters = combinedCharacters.concat([
  2304, 2305, 2306, 2307, 2362, 2363, 2364, 2364, 2366, 2367, 2368, 2369, 2370,
  2371, 2372, 2373, 2374, 2375, 2376, 2377, 2378, 2379, 2380, 2381, 2382, 2383,
  2387, 2388, 2389, 2390, 2391, 2402, 2403,
]);

const BLACK_FLAG_CODE_POINT = 127988;
const CANCEL_TAG_CODE_POINT = 917631;
const A_TAG_CODE_POINT = 917601;
const Z_TAG_CODE_POINT = 917626;
const VARIATION_SELECTOR_16_CODE_POINT = 65039;
const ZERO_WIDTH_JOINER_CODE_POINT = 8205;
const REGIONAL_CHARACTER_A_CODE_POINT = 127462;
const REGIONAL_CHARACTER_Z_CODE_POINT = 127487;

const surrogateModifiers = [
  'd83cdffb',
  'd83cdffc',
  'd83cdffd',
  'd83cdffe',
  'd83cdfff',
];

class TextManager {
  private _text: string;

  constructor(text: string) {
    this._text = text;
  }

  getCodePoint(value: string): number {
    var codePoint = 0;
    var first = value.charCodeAt(0);
    if (first >= 0xd800 && first <= 0xdbff) {
      var second = value.charCodeAt(1);
      if (second >= 0xdc00 && second <= 0xdfff) {
        codePoint = (first - 0xd800) * 0x400 + second - 0xdc00 + 0x10000;
      }
    }
    return codePoint;
  }

  isCombinedCharacter(charCode: number): boolean {
    return combinedCharacters.indexOf(charCode) !== -1;
  }

  // Skin tone modifiers
  isModifier(firstCharCode: number, secondCharCode: number): boolean {
    var sum = firstCharCode.toString(16) + secondCharCode.toString(16);
    return surrogateModifiers.indexOf(sum) !== -1;
  }

  isZeroWidthJoiner(charCode: number): boolean {
    return charCode === ZERO_WIDTH_JOINER_CODE_POINT;
  }

  // This codepoint may change the appearance of the preceding character.
  // If that is a symbol, dingbat or emoji, U+FE0F forces it to be rendered
  // as a colorful image as compared to a monochrome text variant.
  isVariationSelector(charCode: number): boolean {
    return charCode === VARIATION_SELECTOR_16_CODE_POINT;
  }

  // The regional indicator symbols are a set of 26 alphabetic Unicode
  // characters intended to be used to encode ISO 3166-1 alpha-2
  // two-letter country codes in a way that allows optional special treatment.
  isRegionalCode(value: string): boolean {
    var codePoint = this.getCodePoint(value);
    if (
      codePoint >= REGIONAL_CHARACTER_A_CODE_POINT &&
      codePoint <= REGIONAL_CHARACTER_Z_CODE_POINT
    ) {
      return true;
    }
    return false;
  }

  // Some Emoji implementations represent combinations of
  // two "regional indicator" letters as a single flag symbol.
  isFlagEmoji(value: string): boolean {
    return (
      this.isRegionalCode(value.substr(0, 2)) &&
      this.isRegionalCode(value.substr(2, 2))
    );
  }

  // Regional flags start with a BLACK_FLAG_CODE_POINT
  // folowed by 5 chars in the TAG range
  // and end with a CANCEL_TAG_CODE_POINT
  isRegionalFlag(text: string, index: number) {
    var codePoint = this.getCodePoint(text.substr(index, 2));
    if (codePoint !== BLACK_FLAG_CODE_POINT) {
      return false;
    }
    var count = 0;
    index += 2;
    while (count < 5) {
      codePoint = this.getCodePoint(text.substr(index, 2));
      if (codePoint < A_TAG_CODE_POINT || codePoint > Z_TAG_CODE_POINT) {
        return false;
      }
      count += 1;
      index += 2;
    }
    return this.getCodePoint(text.substr(index, 2)) === CANCEL_TAG_CODE_POINT;
  }

  *[Symbol.iterator](): Generator<string> {
    let currentCharIndex: number = 0;
    let shouldCombine: boolean = false;
    let shouldCombineNext: boolean = false;
    let text: string = this._text;
    let currentChars: string = '';
    let charCode: number = 0;
    let secondCharCode: number = 0;
    const charactersArray: string[] = [];
    while (currentCharIndex < text.length) {
      shouldCombine = shouldCombineNext;
      shouldCombineNext = false;
      charCode = text.charCodeAt(currentCharIndex);
      currentChars = text.charAt(currentCharIndex);
      if (this.isCombinedCharacter(charCode)) {
        shouldCombine = true;
        // It's a potential surrogate pair (this is the High surrogate)
      } else if (charCode >= 0xd800 && charCode <= 0xdbff) {
        if (this.isRegionalFlag(text, currentCharIndex)) {
          currentChars = text.substr(currentCharIndex, 14);
        } else {
          secondCharCode = text.charCodeAt(currentCharIndex + 1);
          // It's a surrogate pair (this is the Low surrogate)
          if (secondCharCode >= 0xdc00 && secondCharCode <= 0xdfff) {
            if (this.isModifier(charCode, secondCharCode)) {
              currentChars = text.substr(currentCharIndex, 2);
              shouldCombine = true;
            } else if (this.isFlagEmoji(text.substr(currentCharIndex, 4))) {
              currentChars = text.substr(currentCharIndex, 4);
            } else {
              currentChars = text.substr(currentCharIndex, 2);
            }
          }
        }
      } else if (charCode > 0xdbff) {
        secondCharCode = text.charCodeAt(currentCharIndex + 1);
        if (this.isVariationSelector(charCode)) {
          shouldCombine = true;
        }
      } else if (this.isZeroWidthJoiner(charCode)) {
        shouldCombine = true;
        shouldCombineNext = true;
      }
      if (shouldCombine) {
        charactersArray[charactersArray.length - 1] += currentChars;
        shouldCombine = false;
      } else {
        charactersArray.push(currentChars);
      }
      currentCharIndex += currentChars.length;
    }
    for (let i = 0; i < charactersArray.length; i += 1) {
      yield charactersArray[i];
    }
  }
}

const factory = (text: string) => {
  return new TextManager(text);
};

export default factory;
