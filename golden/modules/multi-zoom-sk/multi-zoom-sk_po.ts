import {PageObject} from '../../../infra-sk/modules/page_object/page_object';
import {PageObjectElement} from '../../../infra-sk/modules/page_object/page_object_element';
import {CheckOrRadio} from 'elements-sk/checkbox-sk/checkbox-sk';

/** A page object for the MultiZoomSk component. */
export class MultiZoomSkPO extends PageObject {
  private get leftCheckbox(): PageObjectElement {
    return this.bySelector('checkbox-sk.idx_0');
  }

  private get diffCheckbox(): PageObjectElement {
    return this.bySelector('checkbox-sk.idx_1');
  }

  private get rightCheckbox(): PageObjectElement {
    return this.bySelector('checkbox-sk.idx_2');
  }

  private get coordinate(): PageObjectElement {
    return this.bySelector('table.stats td.coord');
  }

  private get leftPixel(): PageObjectElement {
    return this.bySelector('table.stats td.left');
  }

  private get diffPixel(): PageObjectElement {
    return this.bySelector('table.stats td.diff');
  }

  private get rightPixel(): PageObjectElement {
    return this.bySelector('table.stats td.right');
  }

  private get sizeWarning(): PageObjectElement {
    return this.bySelector('size_warning');
  }

  private get nthDiff(): PageObjectElement {
    return this.bySelector('.nth_diff');
  }

  isLeftCheckboxChecked(): Promise<boolean> {
    return this.leftCheckbox.applyFnToDOMNode((el) => (el as CheckOrRadio).checked);
  }

  async clickLeftCheckbox() { await this.leftCheckbox.click(); }

  isDiffCheckboxChecked(): Promise<boolean> {
    return this.diffCheckbox.applyFnToDOMNode((el) => (el as CheckOrRadio).checked);
  }

  async clickDiffCheckbox() { await this.diffCheckbox.click(); }

  isRightCheckboxChecked(): Promise<boolean> {
    return this.rightCheckbox.applyFnToDOMNode((el) => (el as CheckOrRadio).checked);
  }

  async clickRightCheckbox() { await this.rightCheckbox.click(); }

  isLeftDisplayed(): Promise<boolean> { return this.leftCheckbox.hasClassName('displayed'); }

  isDiffDisplayed(): Promise<boolean> { return this.diffCheckbox.hasClassName('displayed'); }

  isRightDisplayed(): Promise<boolean> { return this.rightCheckbox.hasClassName('displayed'); }

  getCoordinate(): Promise<string> { return this.coordinate.innerText; }

  getLeftPixel(): Promise<string> { return this.leftPixel.innerText; }

  getDiffPixel(): Promise<string> { return this.diffPixel.innerText; }

  getRightPixel(): Promise<string> { return this.rightPixel.innerText; }

  async isSizeWarningVisible(): Promise<boolean> { return !(await this.sizeWarning.isEmpty()); }

  async isNthDiffVisible(): Promise<boolean> { return !(await this.nthDiff.isEmpty()); }

  getNthDiff(): Promise<string> { return this.nthDiff.innerText; }

  async getDisplayedImage(): Promise<'left' | 'diff' | 'right'> {
    let numDisplayedImages = 0;
    let displayedImage: 'left' | 'diff' | 'right';
    if (await this.isLeftDisplayed()) {
      displayedImage = 'left';
      numDisplayedImages++;
    }
    if (await this.isDiffDisplayed()) {
      displayedImage = 'diff';
      numDisplayedImages++;
    }
    if (await this.isRightDisplayed()) {
      displayedImage = 'right';
      numDisplayedImages++;
    }
    if (numDisplayedImages !== 1) {
      throw new Error(
          `Expected 1 image to be displayed, was: ${numDisplayedImages}. This is a bug.`);
    }
    return displayedImage!;
  }

  async sendKeypress(key: string) { await this.element.typeKey(key); }
}