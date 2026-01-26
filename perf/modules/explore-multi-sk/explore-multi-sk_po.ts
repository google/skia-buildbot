import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { TestPickerSkPO } from '../test-picker-sk/test-picker-sk_po';
import { ExploreSimpleSkPO } from '../explore-simple-sk/explore-simple-sk_po';
import { poll } from '../common/puppeteer-test-util';

export class ExploreMultiSkPO extends PageObject {
  get testPicker(): TestPickerSkPO {
    return new TestPickerSkPO(this.bySelector('test-picker-sk'));
  }

  /**
   * Returns the first graph (kept for backward compatibility).
   */
  get exploreSimpleSk(): ExploreSimpleSkPO {
    return this.getGraph(0);
  }

  /**
   * Returns the graph Page Object at the specific index (0-based).
   * Note: We use nth-of-type (1-based) selector logic.
   */
  getGraph(index: number): ExploreSimpleSkPO {
    return new ExploreSimpleSkPO(this.bySelector(`explore-simple-sk:nth-of-type(${index + 1})`));
  }

  /**
   * Returns the total number of explore-simple-sk graphs currently in the DOM.
   */
  async getGraphCount(): Promise<number> {
    // We access the host element and count the children matching the tag.
    return await this.element.applyFnToDOMNode((el) => {
      return el.querySelectorAll('explore-simple-sk').length;
    });
  }

  /**
   * Waits until the specific number of graphs are present in the DOM.
   */
  async waitForGraphCount(expectedCount: number): Promise<void> {
    await poll(
      async () => (await this.getGraphCount()) === expectedCount,
      `Waiting for graph count to be ${expectedCount}`
    );
  }

  /**
   * Waits for a specific graph to be fully loaded (displaying the plot).
   * We use bySelector directly here to access applyFnToDOMNode.
   * * @param index The index of the graph to wait for (default: 0)
   * * @param timeoutMs Optional timeout in milliseconds.
   */
  async waitForGraph(index: number = 0, timeoutMs?: number): Promise<void> {
    await poll(
      async () => {
        const graphElement = this.bySelector(`explore-simple-sk:nth-of-type(${index + 1})`);

        if (await graphElement.isEmpty()) return false;

        return await graphElement.applyFnToDOMNode((el) => {
          const root = el.shadowRoot || el;
          const exploreDiv = root.querySelector('#explore');
          return exploreDiv ? exploreDiv.classList.contains('display_plot') : false;
        });
      },
      `Waiting for graph [${index}] to load (display_plot class)`,
      timeoutMs
    );
  }
}
