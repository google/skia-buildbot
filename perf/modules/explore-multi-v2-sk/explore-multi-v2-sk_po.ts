import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';
import { poll, waitForElementVisible } from '../common/puppeteer-test-util';

/** A page object for the explore-multi-v2-sk component. */
export class ExploreMultiV2SkPO extends PageObject {
  get staticContent() {
    return this.element.applyFnToDOMNode(async (el) => {
      const explore = el;
      if (!explore || !explore.shadowRoot) {
        return null;
      }
      const h1 = explore.shadowRoot.querySelector('h1');
      const p = explore.shadowRoot.querySelector('p.subtitle');
      const sectionTitles = Array.from(explore.shadowRoot.querySelectorAll('.section-title')).map(
        (elem) => elem.textContent?.trim()
      );

      return {
        title: h1 ? h1.textContent?.trim() : null,
        subtitle: p ? p.textContent?.trim() : null,
        facetedSearchBarTitle: sectionTitles[0] || null,
        visualizationsTitle: sectionTitles[1] || null,
      };
    });
  }

  get addQueryButton(): PageObjectElement {
    return this.bySelector('.add-query-circle-btn');
  }

  get diffBaseChip(): PageObjectElement {
    return this.bySelector('.config-pill.diff-base');
  }

  get queryBars(): PageObjectElementList {
    return this.bySelectorAll('query-bar-sk');
  }

  async getQueryBarCount(): Promise<number> {
    return await this.element.applyFnToDOMNode((el) => {
      return el.shadowRoot?.querySelectorAll('query-bar-sk').length || 0;
    });
  }

  async isWorkerReady(): Promise<boolean> {
    return await this.element.applyFnToDOMNode(async (el: any) => {
      for (let i = 0; i < 20; i++) {
        if (el._workerController && el._workerController.isReady()) return true;
        await new Promise((resolve) => setTimeout(resolve, 50));
      }
      return false;
    });
  }

  async getSuggestionCountText(): Promise<string> {
    return await this.element.applyFnToDOMNode(async (el: any) => {
      const queryBar = el.shadowRoot.querySelector('query-bar-sk') as any;
      const countEl = queryBar.shadowRoot.querySelector('.s-count.right');
      return countEl ? countEl.textContent : '';
    });
  }

  async getDiffBase(): Promise<{ key: string; value: string } | null> {
    return await this.element.applyFnToDOMNode(async (el: any) => {
      return el._diffBase;
    });
  }

  async clickDiffButtonOnFirstQueryBarPill(): Promise<void> {
    return await this.element.applyFnToDOMNode(async (el: any) => {
      const queryBar = el.shadowRoot.querySelector('query-bar-sk') as any;
      let multiSelect = queryBar.shadowRoot.querySelector('explore-multi-v2-select-sk') as any;
      for (let i = 0; i < 20 && !multiSelect; i++) {
        await new Promise((resolve) => setTimeout(resolve, 50));
        multiSelect = queryBar.shadowRoot.querySelector('explore-multi-v2-select-sk');
      }
      if (!multiSelect) throw new Error('explore-multi-v2-select-sk not found');
      multiSelect._isOpen = true;

      // Wait for options to be populated and the button to render
      const maxAttempts = 20;
      for (let attempt = 0; attempt < maxAttempts; attempt++) {
        await multiSelect.updateComplete;
        const diffBtn = multiSelect.shadowRoot.querySelector('.ms-diff-btn') as HTMLElement;
        if (diffBtn) {
          diffBtn.click();
          return;
        }
        await new Promise((resolve) => setTimeout(resolve, 50));
      }
      throw new Error('Diff button (.ms-diff-btn) did not appear in the multi-select dropdown!');
    });
  }

  async waitForDiffBaseChip(): Promise<void> {
    await poll(async () => {
      const chip = this.diffBaseChip;
      return !(await chip.isEmpty()) && (await chip.innerText).includes('Diff Base:');
    }, 'Diff base chip did not become visible with the correct text.');
  }

  async waitForAddQueryButton(): Promise<void> {
    await waitForElementVisible(this.addQueryButton, 'Add query button did not become visible');
  }
}
