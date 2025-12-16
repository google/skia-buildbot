import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { TestPickerSkPO } from '../test-picker-sk/test-picker-sk_po';
import { ExploreSimpleSkPO } from '../explore-simple-sk/explore-simple-sk_po';
import { poll } from '../common/puppeteer-test-util';

export class ExploreMultiSkPO extends PageObject {
  get testPicker(): TestPickerSkPO {
    return new TestPickerSkPO(this.bySelector('test-picker-sk'));
  }

  get exploreSimpleSk(): ExploreSimpleSkPO {
    return new ExploreSimpleSkPO(this.bySelector('explore-simple-sk'));
  }

  async waitForGraph(): Promise<void> {
    await poll(async () => {
      const exploreSimple = this.bySelector('explore-simple-sk');
      if (await exploreSimple.isEmpty()) return false;

      return await exploreSimple.applyFnToDOMNode((el) => {
        const root = el.shadowRoot || el;
        const exploreDiv = root.querySelector('#explore');
        return exploreDiv ? exploreDiv.classList.contains('display_plot') : false;
      });
    }, 'Waiting for graph to load (display_plot class)');
  }
}
