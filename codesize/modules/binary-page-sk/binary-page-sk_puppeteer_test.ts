import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';
import { END_BUSY_EVENT } from '../codesize-scaffold-sk/events';

describe('binary-page-sk', () => {
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
    expect(await testBed.page.$$('binary-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'codesize', 'binary-page-sk');
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

      await takeScreenshot(testBed.page, 'codesize', 'binary-page-sk_node');
    });

    it('shows some suggestions when the user types in letters', async () => {
      // https://stackoverflow.com/a/56772379/1447621
      await testBed.page.focus('.search-bar input');
      await testBed.page.keyboard.type('s');

      await takeScreenshot(testBed.page, 'codesize', 'binary-page-sk-search_bar');
    });

    it('jumps to the first result when the user types enter', async () => {
      await testBed.page.focus('.search-bar input');
      await testBed.page.keyboard.type('skp');
      await testBed.page.keyboard.press('Enter');

      await takeScreenshot(testBed.page, 'codesize', 'binary-page-sk-search_bar_enter');
    });
  });
});
