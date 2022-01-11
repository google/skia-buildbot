import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

import { exampleTraceString } from './demo_data';

describe('debugger-app-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1024, height: 1024 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('debugger-app-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'shaders', 'debugger-app-sk');
    });

    it('responds to a drop event by loading the file', async () => {
      const dragArea = await testBed.page.$('#drag-area');
      await dragArea!.evaluate((ele: Element, traceString: string) => {
        // We cannot make a DragEvent because the dataTransfer field is readonly (it's controlled
        // by the browser and relates to actual files). Instead, we can make a custom event
        // and add a dataTransfer field to it. The "file" is just a blob with the text content
        // that we want to be read in (the JSON of our trace).
        const de = new CustomEvent('drop');
        (de as any).dataTransfer = {
          files: [new Blob([traceString], { type: 'text/plain' })],
        };
        ele.dispatchEvent(de);
      }, exampleTraceString);
      await takeScreenshot(testBed.page, 'shaders', 'debugger-app-sk_drop');
    });
  });
});
