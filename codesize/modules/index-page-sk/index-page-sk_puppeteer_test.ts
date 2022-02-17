import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';
import { END_BUSY_EVENT } from '../codesize-scaffold-sk/events';

describe('index-page-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.setViewport({ width: 1024, height: 768 });
    const eventPromise = await addEventListenersToPuppeteerPage(testBed.page, [END_BUSY_EVENT]);
    const loaded = eventPromise(END_BUSY_EVENT); // Emitted when page is loaded.
    await testBed.page.goto(testBed.baseUrl);
    await loaded;
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('index-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'codesize', 'index-page-sk');
    });

    it('shows a child node', async () => {
      // Hover over the rightmost node in the tree. We use coordinates because the SVG generated
      // by the google.visualization.TreeMap is not amenable to querying.
      const treemap = await testBed.page.$('#treemap');
      const box = await treemap.boundingBox();
      await testBed.page.mouse.click(
        box.x + box.width - 50,
        box.y + box.height - 50,
      );

      // Give the TreeMap a chance to redraw.
      await new Promise((resolve) => setTimeout(resolve, 1000));

      await takeScreenshot(testBed.page, 'codesize', 'index-page-sk_node');
    });
  });
});
