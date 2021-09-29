import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement, PageObjectElementList } from '../../../infra-sk/modules/page_object/page_object_element';
import { Label } from '../rpc_types';

/** A page element for the DotsLegendSk compoment. */
export class DotsLegendSkPO extends PageObject {
  private get dots(): PageObjectElementList {
    return this.bySelectorAll('div.dot');
  }

  private get digests(): PageObjectElementList {
    return this.bySelectorAll('a.digest, span.one-of-many-other-digests');
  }

  private get digestLinks(): PageObjectElementList {
    return this.bySelectorAll('a.digest');
  }

  private get diffLinks(): PageObjectElementList {
    return this.bySelectorAll('a.diff');
  }

  private get triageStatusIcons(): PageObjectElementList {
    return this.bySelectorAll('.positive-icon, .negative-icon, .untriaged-icon');
  }

  /**
   * Returns a [border color, background color] tuple for each dot, where each color is represented
   * as a hex string (e.g. "#0ABBCC").
   */
  getDotBorderAndBackgroundColors(): Promise<[string, string][]> {
    return this.dots.map(async (dot: PageObjectElement) => [
      rgbToHex(await dot.applyFnToDOMNode((el: HTMLElement) => el.style.borderColor)),
      rgbToHex(await dot.applyFnToDOMNode((el: HTMLElement) => el.style.backgroundColor)),
    ]);
  }

  getDigests(): Promise<string[]> {
    return this.digests.map((digest) => digest.innerText);
  }

  getDigestHrefs(): Promise<(string | null)[]> {
    return this.digestLinks.map((a) => a.getAttribute('href'));
  }

  getDiffHrefs(): Promise<(string | null)[]> {
    return this.diffLinks.map((a) => a.getAttribute('href'));
  }

  getTriageIconLabels(): Promise<Label[]> {
    return this.triageStatusIcons.map(async (icon) => {
      const className = await icon.className;
      switch (className) {
        case 'positive-icon': return 'positive';
        case 'negative-icon': return 'negative';
        case 'untriaged-icon': return 'untriaged';
      }
      throw new Error(`Unknown triage icon class: ${className}`);
    });
  }
}

// Takes a color represented as an RGB string (e.g. "rgb(10, 187, 204)") and
// returns the equivalent hex string (e.g. "#0ABBCC").
const rgbToHex = (rgb: string): string => `#${rgb.match(/rgb\((\d+), (\d+), (\d+)\)/)!
  .slice(1) // ['10', '187', '204'].
  .map((x: string) => parseInt(x)) // [10, 187, 204]
  .map((x: number) => x.toString(16)) // ['a', 'bb', 'cc']
  .map((x: string) => x.padStart(2, '0')) // ['0a', 'bb', 'cc']
  .map((x: string) => x.toUpperCase()) // ['0A', 'BB', 'CC']
  .join('')}`; // '0ABBCC'
