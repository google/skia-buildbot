import { BugTooltipSkPO } from '../bug-tooltip-sk/bug-tooltip-sk_po';
import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { AnomaliesTableSkPO } from './anomalies-table-sk_po';
import { anomaly_table, associatedBugs } from './test_data';
import { ElementHandle } from 'puppeteer';
import { Page } from 'puppeteer';
import { assert } from 'chai';
import { TriageMenuSkPO } from '../triage-menu-sk/triage-menu-sk_po';
import { ExistingBugDialogSkPO } from '../existing-bug-dialog-sk/existing-bug-dialog-sk_po';

describe('anomalies-table-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });
  let anomaliesTableSk: ElementHandle;
  let anomaliesTableSkPO: AnomaliesTableSkPO;

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    anomaliesTableSk = (await testBed.page.$('anomalies-table-sk'))!;
    anomaliesTableSkPO = new AnomaliesTableSkPO(anomaliesTableSk);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('anomalies-table-sk')).to.have.length(2); // Smoke test.
  });

  describe('with anomalies', () => {
    beforeEach(async () => {
      await testBed.page.click('#populate-tables');
    });

    it('shows the default view', async () => {
      const rowCount = await anomaliesTableSkPO.getRowCount();
      expect(rowCount).to.be.greaterThanOrEqual(anomaly_table.length);
    });

    it('should be able to scroll up and down', async () => {
      // Scroll down by 1000px.
      await testBed.page.evaluate(() => window.scrollBy(0, 1000));
      const scrollYAfterScrollDown = await testBed.page.evaluate(() => window.scrollY);
      expect(scrollYAfterScrollDown).to.be.greaterThan(0);

      // Scroll up by 1000px.
      await testBed.page.evaluate(() => window.scrollBy(0, -1000));
      const scrollYAfterScrollUp = await testBed.page.evaluate(() => window.scrollY);
      expect(scrollYAfterScrollUp).to.equal(0);
    });

    it('should be able to hide and show rows', async () => {
      // Initially, the child row should be hidden.
      expect(await anomaliesTableSkPO.getChildRowCount()).to.equal(0);

      // Click the expand button of the first group.
      await anomaliesTableSkPO.clickExpandButton(0);
      expect(await anomaliesTableSkPO.getChildRowCount()).to.be.greaterThan(0);

      // Click it again to collapse.
      await anomaliesTableSkPO.clickExpandButton(0);
      expect(await anomaliesTableSkPO.getChildRowCount()).to.equal(0);
    });

    it('should be able to expand collapsed rows', async () => {
      const initialRowCount = await anomaliesTableSkPO.getRowCount();
      await anomaliesTableSkPO.clickExpandButton(0);
      const expandedRowCount = await anomaliesTableSkPO.getRowCount();
      // Expect 2 additional rows (group size 2).
      expect(expandedRowCount).to.be.equal(initialRowCount + 2);
    });

    it('should be able to click triage button once it clicks one row', async () => {
      await anomaliesTableSkPO.clickCheckbox(1);
      await anomaliesTableSkPO.clickTriageButton();
      const triageMenu = await testBed.page.$('triage-menu-sk');
      assert.isNotNull(triageMenu);
    });

    it('should be able to click New Bug button after one row checkbox is selected', async () => {
      await anomaliesTableSkPO.clickCheckbox(0);
      await anomaliesTableSkPO.clickTriageButton();
      const triageMenuSk = await testBed.page.$('triage-menu-sk');
      assert.isNotNull(triageMenuSk);

      const triageMenuSkPO = new TriageMenuSkPO(triageMenuSk!);
      await triageMenuSkPO.newBugButton.click();
      const newBugDialog = await testBed.page.$('new-bug-dialog-sk');
      assert.isNotNull(newBugDialog);
    });

    it('should be able to click Existing Bug button after one checkbox is selected', async () => {
      await anomaliesTableSkPO.clickCheckbox(0);
      await anomaliesTableSkPO.clickTriageButton();
      const triageMenuSk = await testBed.page.$('triage-menu-sk');
      assert.isNotNull(triageMenuSk);

      const triageMenuSkPO = new TriageMenuSkPO(triageMenuSk!);
      await triageMenuSkPO.existingBugButton.click();
      const existingBugDialog = await testBed.page.$('existing-bug-dialog-sk');
      assert.isNotNull(existingBugDialog);
    });

    it('should be able to click Existing Bug button and display mock data', async () => {
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);
      await anomaliesTableSkPO.clickCheckbox(1);
      await anomaliesTableSkPO.clickTriageButton();
      const triageMenu = await testBed.page.$('triage-menu-sk');
      const triageMenuSkPO = new TriageMenuSkPO(triageMenu!);
      await triageMenuSkPO.existingBugButton.click();

      const existingBugDialog = await testBed.page.$('existing-bug-dialog-sk');
      assert.isNotNull(existingBugDialog);

      const existingBugDialogSkPO = new ExistingBugDialogSkPO(existingBugDialog!);
      // Assert that the mocked bug ID and title are displayed
      const associatedBugLinks = existingBugDialogSkPO.associatedBugLinks;

      const validBugIds = ['12345', '23456', '34567', '-1'];
      for (let i = 0; i < (await associatedBugLinks.length); i++) {
        const text = await (await associatedBugLinks.item(i)).innerText;
        const matches = validBugIds.some((id) => text.includes(id));
        expect(
          matches,
          `Expected dialog to include one of ${validBugIds.join(', ')}, but found '${text}'`
        ).to.be.true;
      }
      const itemSelectorUrls = '#associated-bugs-table li a';
      testBed.page.$$eval(itemSelectorUrls, (items) => {
        const bugUrls = items.map((item) => {
          const linkElement = item.querySelector('a');
          const titleElement = item.querySelector('#bug-title');
          return {
            id: linkElement ? linkElement.textContent || '' : '',
            url: linkElement ? (linkElement as HTMLAnchorElement).href : '',
            title: titleElement ? titleElement.textContent || '' : '',
          };
        });

        expect(bugUrls.length).to.equal(2);
        for (let i = 0; i < bugUrls.length; i++) {
          const expected = associatedBugs[i];
          const actual = bugUrls[i];
          expect(actual.id).equals(expected.id);
          expect(actual.url).equals(expected.url);
          expect(actual.title).equals(expected.title);
        }
      });
    });

    it('should redirect to Buganizer when New Bug button is clicked', async () => {
      await anomaliesTableSkPO.clickCheckbox(0);
      await anomaliesTableSkPO.clickTriageButton();
      const triageMenuSk = await testBed.page.$('triage-menu-sk');
      assert.isNotNull(triageMenuSk);

      const triageMenuSkPO = new TriageMenuSkPO(triageMenuSk!);
      await triageMenuSkPO.newBugButton.click(); // Open the new bug dialog

      const newBugDialog = await testBed.page.$('new-bug-dialog-sk');
      assert.isNotNull(newBugDialog);

      // Fill a required input in the new bug dialog
      await testBed.page.type('new-bug-dialog-sk #title', 'Test Bug Title');

      // Start waiting for the popup and call fileNewBug() on the element
      const [buganizerPage] = await Promise.all([
        new Promise<Page>((resolve) => testBed.page.once('popup', resolve)),
        testBed.page.evaluate((el: any) => el.fileNewBug(), newBugDialog),
      ]);

      assert.isNotNull(buganizerPage);
      expect(buganizerPage.url()).to.include('https://issues.chromium.org/issues/358011161'); // Check for the mocked bug_id
      await buganizerPage.close(); // Close the new tab
    });

    it('should be able to click expand checkbox', async () => {
      await anomaliesTableSkPO.clickExpandButton(0);
      const summaryRowCount: number = await anomaliesTableSkPO.getParentExpandRowCount();
      expect(summaryRowCount).to.equal(1);
    });

    it('should display the correct bug ids', async () => {
      const bugLinks = await anomaliesTableSkPO.bugLinks;
      const bugId = await (await bugLinks.item(0)).innerText;

      expect(bugId).to.equal(anomaly_table[0].bug_id.toString());
    });

    it('should have the correct bug link hrefs', async () => {
      const bugLinks = await anomaliesTableSkPO.bugLinks;
      const link: string = (await (await bugLinks.item(0)).getAttribute('href'))!;
      expect(link).to.equal(`http://b/${anomaly_table[0].bug_id}`);
    });

    it('opens a new tab with the correct URL for the trending icon', async () => {
      await anomaliesTableSkPO.clickCheckbox(1);
      const openMultiGraphUrl = async () => await testBed.page.click('#open-multi-graph');

      // Start waiting for the popup and click the link at the same time.
      const [popup] = await Promise.all([
        new Promise<Page>((resolve) => testBed.page.once('popup', resolve)),
        openMultiGraphUrl(),
      ]);
      assert.isNotNull(popup);
    });

    it('opens a new report page with the correct URL for a multiple anomalies', async () => {
      await anomaliesTableSkPO.clickExpandButton(0);
      await anomaliesTableSkPO.clickCheckbox(0);
      await anomaliesTableSkPO.clickCheckbox(1);

      await anomaliesTableSkPO.clickGraphButton();
      const reportPageUrl = await navigateTo(testBed.page, testBed.baseUrl, `/u/?sid=test-sid`);
      assert.exists(reportPageUrl);
    });
  });

  describe('bug tooltip', () => {
    let bugTooltipSkPO: BugTooltipSkPO;
    let bugTooltipHandle: ElementHandle<Element>;

    const hoverOnBugTooltip = async () => {
      await bugTooltipSkPO.hoverOverBugCountContainer();
    };

    const waitForBugTooltip = async () => {
      await testBed.page.waitForSelector('bug-tooltip-sk');
      bugTooltipHandle = (await testBed.page.$('bug-tooltip-sk'))!;
      bugTooltipSkPO = new BugTooltipSkPO(bugTooltipHandle);
      await testBed.page.waitForFunction(
        () =>
          !document
            .querySelector('bug-tooltip-sk')
            ?.querySelector('.bug-count-container')
            ?.hasAttribute('hidden')
      );
    };

    it('shows the tooltip on hover', async () => {
      await testBed.page.click('#populate-tables-it');
      await waitForBugTooltip();
      await hoverOnBugTooltip();
      expect(await bugTooltipSkPO.isTooltipVisible()).to.be.true;
    });

    it('tooltip scrollable if many items are present', async () => {
      await testBed.page.click('#populate-tables-it');
      await waitForBugTooltip();
      await hoverOnBugTooltip();
      const content = await bugTooltipSkPO.getContent();
      expect(content).to.contain('12345');
      expect(content).to.contain('67890');
      expect(content).to.contain('11121');
      expect(content).to.contain('11122');
      expect(content).to.contain('11123');
      expect(content).to.contain('11124');
      expect(await bugTooltipSkPO.isScrollable()).to.be.true;
    });

    it('tooltip not scrollable if few items are present', async () => {
      await testBed.page.click('#populate-tables-it1');
      await waitForBugTooltip();
      await hoverOnBugTooltip();
      const content = await bugTooltipSkPO.getContent();
      expect(content).to.contain('54321');
      expect(await bugTooltipSkPO.isScrollable()).to.be.false;
    });

    it('no text is shown if there are no bugs', async () => {
      await testBed.page.click('#populate-tables-it2');
      await testBed.page.waitForSelector('bug-tooltip-sk');
      bugTooltipHandle = (await testBed.page.$('bug-tooltip-sk'))!;
      bugTooltipSkPO = new BugTooltipSkPO(bugTooltipHandle);
      const visible = await bugTooltipSkPO.isBugContainerVisible();
      expect(visible).to.equal(false);
    });
  });

  describe('open report page with single anomaly id', async () => {
    beforeEach(async () => {
      await testBed.page.click('#populate-tables');
    });

    it('opens a new report page with the correct URL for single anomaly', async () => {
      await anomaliesTableSkPO.clickExpandButton(0);
      await anomaliesTableSkPO.clickCheckbox(0);
      await anomaliesTableSkPO.clickCheckbox(1);

      await anomaliesTableSkPO.clickGraphButton();
      const reportPageUrl = await navigateTo(testBed.page, testBed.baseUrl, `/u/?anomalyIDs=1`);
      assert.exists(reportPageUrl);
    });
  });

  describe('grouping configuration', () => {
    beforeEach(async () => {
      await testBed.page.click('#populate-tables-for-grouping');

      await anomaliesTableSkPO.setRevisionMode('OVERLAPPING');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', true);
      await anomaliesTableSkPO.setGroupBy('BOT', false);
      await anomaliesTableSkPO.setGroupBy('TEST', false);
      await anomaliesTableSkPO.setGroupSingles(false);
    });

    afterEach(async () => {
      await takeScreenshot(testBed.page, 'perf', 'anomalies-table-sk_grouping');
    });

    it('1. Revision: EXACT | GroupBy: NONE', async () => {
      await anomaliesTableSkPO.setRevisionMode('EXACT');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);

      // Groups:
      // 1. Bug 12345 (merged) -> 1 Group.
      // 2. Rev A (3 items) -> 1 Group (Multi-item).
      // 3. Rev B (1 item) -> 1 Group (Single).
      // 4. Single 1 (1 item) -> 1 Group.
      // 5. Single 2 (1 item) -> 1 Group.
      // Total Groups = 5.
      // Total Rows = 5 Groups + 1 Header Row (normalized by browser) = 6 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(6);
    });

    it('2. Revision: OVERLAPPING | GroupBy: NONE', async () => {
      await anomaliesTableSkPO.setRevisionMode('OVERLAPPING');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);

      // Groups:
      // 1. Bug 12345 (merged) -> 1 Group.
      // 2. Rev A (3 items) -> 1 Group (Exclude from overlap check).
      // 3. Rev B (1 item) -> 1 Group (Matches EXACT first, separate from Rev A).
      // 4. Single 1 (1 item) -> 1 Group.
      // 5. Single 2 (1 item) -> 1 Group.
      // Total Groups = 5.
      // Total Rows = 5 Groups + 1 Header Row = 6 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(6);
    });

    it('3. Revision: ANY | GroupBy: NONE', async () => {
      await anomaliesTableSkPO.setRevisionMode('ANY');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);

      // Groups:
      // 1. Bug 12345 (merged) -> 1 Group.
      // 2. All others (6 items) -> 1 Revision Group (due to ANY mode).
      // Total Groups = 2.
      // Total Rows = 2 Groups + 1 Header Row = 3 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(3);
    });

    it('4. Revision: ANY | GroupBy: BOT', async () => {
      await anomaliesTableSkPO.setRevisionMode('ANY');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);
      await anomaliesTableSkPO.setGroupBy('BOT', true);
      // Logic:
      // 1. Bug 12345 Group (merged) -> 1 Row.
      // 2. Revision Group (ANY) splits by BOT:
      //    - BotA Group (5 items) -> 1 Row.
      //    - BotB Group (1 item) -> 1 Row.
      // Total = 1 Bug + 1 BotA + 1 BotB + 1 Header = 4 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(4);
    });

    it('5. Revision: ANY | GroupBy: BENCHMARK', async () => {
      await anomaliesTableSkPO.setRevisionMode('ANY');
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(4);
      // Logic:
      // 1. Bug 12345 Group (merged) -> 1 Group.
      // 2. Revision Group (ANY) contains 6 items.
      //    Split by BENCHMARK:
      //    - BenchX (Rev A 1,2,3; Rev B 1) -> 1 Group.
      //    - BenchZ (Single 1,2) -> 1 Group.
      // Total Groups = 3 (Bug + BenchX + BenchZ).
      // Total Rows = 3 Groups + 1 Header Row = 4 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(4);
    });

    it('6. GroupSingles: TRUE', async () => {
      await anomaliesTableSkPO.setRevisionMode('EXACT');
      await anomaliesTableSkPO.setGroupSingles(true);

      // Groups:
      // 1. Bug 12345 (merged) -> 1 Group.
      // 2. Rev A (Multi-item) -> 1 Group.
      // 3. Rev B (Single) matches BenchX.
      // 4. Single 1 matches BenchZ.
      // 5. Single 2 matches BenchZ.
      // GroupSingles=TRUE (default criteria: BENCHMARK).
      // - BenchX Group: Rev B (1 item).
      // - BenchZ Group: Single 1 + Single 2 (2 items).
      // Total Groups = 1 Bug + 1 Rev A + 1 RevB(BenchX) + 1 Singles(BenchZ) = 4 Groups.
      // Total Rows = 4 Groups + 1 Header Row = 5 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(5);
    });
  });

  async function navigateTo(
    page: Page,
    base: string,
    queryParams = ''
  ): Promise<AnomaliesTableSkPO> {
    await page.goto(`${base}${queryParams}`);
    return new AnomaliesTableSkPO(page.$('anomalies-table-sk'));
  }
});
