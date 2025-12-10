import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { errorMessage } from '../errorMessage';

export class ExploreSimpleSkPO extends PageObject {
  get googleChart(): PageObjectElement {
    return this.bySelector('plot-google-chart-sk');
  }

  get plotSummary(): PageObjectElement {
    return this.bySelector('plot-summary-sk');
  }

  get xAxisSwitch(): PageObjectElement {
    return this.bySelector('#commit-switch');
  }

  async getXAxisDomain(): Promise<string> {
    return await this.googleChart.applyFnToDOMNode((c: any) => c.domain);
  }

  get xAxisSwitchSelector(): string {
    // Replace with the actual CSS selector for the switch element
    // within the explore-simple-sk component.
    return '#commit-switch';
  }

  get getXAxisSwitch(): PageObjectElement {
    return this.bySelector(this.xAxisSwitchSelector);
  }

  async clickXAxisSwitch(): Promise<void> {
    try {
      // Wait for the element to be in the DOM and visible
      await this.bySelector(this.xAxisSwitchSelector);
    } catch (e) {
      await errorMessage(e as Error);
    }

    const switchEl = await this.getXAxisSwitch;
    if (!switchEl) {
      throw new Error('X-Axis switch element not found after visibility wait.');
    }

    // The element is visible, now attempt the click.
    try {
      await switchEl.click();
    } catch (e) {
      // Optional: Add more debug info if click still fails
      console.error('Error clicking the x-axis switch:', e);
      throw e;
    }
  }
}
