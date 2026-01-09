import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';

export class SidePanelSkPO extends PageObject {
  get toggleButton(): PageObjectElement {
    return this.bySelectorShadow('.show-hide-bar');
  }

  get infoPanel(): PageObjectElement {
    return this.bySelectorShadow('.info');
  }

  get selectAllCheckbox(): PageObjectElement {
    return this.bySelectorShadow('#header-checkbox');
  }

  get legendItems(): PageObjectElementList {
    return this.bySelectorShadow('#rows').bySelectorAll('ul li');
  }

  async getLegendItem(index: number): Promise<LegendItemPO> {
    const item = this.legendItems.item(index);
    return new LegendItemPO(await item);
  }

  async isPanelOpen(): Promise<boolean> {
    return await this.infoPanel.applyFnToDOMNode((el) => !el.classList.contains('closed'));
  }

  async clickToggle(): Promise<void> {
    await this.toggleButton.click();
  }

  async clickSelectAll(): Promise<void> {
    await this.selectAllCheckbox.click();
  }
}

export class LegendItemPO extends PageObject {
  get checkbox(): PageObjectElement {
    return this.bySelector('input[type="checkbox"]');
  }

  get label(): PageObjectElement {
    return this.bySelector('label');
  }

  async getDisplayName(): Promise<string> {
    return this.label.applyFnToDOMNode((el) => {
      // Find the text node that is a direct child of the label.
      if (!el.childNodes) {
        return '';
      }
      for (let i = 0; i < el.childNodes.length; i++) {
        const node = el.childNodes[i];
        if (node.nodeType === Node.TEXT_NODE) {
          return node.textContent?.trim() || '';
        }
      }
      return '';
    });
  }

  async isChecked(): Promise<boolean> {
    return this.checkbox.applyFnToDOMNode((el) => (el as HTMLInputElement).checked);
  }

  async isDisabled(): Promise<boolean> {
    return this.checkbox.applyFnToDOMNode((el) => (el as HTMLInputElement).disabled);
  }

  async isHighlighted(): Promise<boolean> {
    return this.label.applyFnToDOMNode((el) => el.classList.contains('highlight'));
  }

  async clickCheckbox(): Promise<void> {
    await this.checkbox.click();
  }
}
