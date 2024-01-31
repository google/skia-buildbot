import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('alert-config-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 2000, height: 2500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('alert-config-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'alert-config-sk');
    });
    it('does not show group_by if window.perf.display_group_by is false', async () => {
      await testBed.page.click('#hide_group_by');
      await takeScreenshot(testBed.page, 'perf', 'alert-config-sk-no-group-by');
    });
    it('shows email control if window.perf.notification is html_email', async () => {
      await testBed.page.click('#display_email');
      await takeScreenshot(testBed.page, 'perf', 'alert-config-sk-email');
    });
    it('shows issue tracker control if window.perf.notification is markdown_issuetracker', async () => {
      await testBed.page.click('#display_issue');
      await takeScreenshot(
        testBed.page,
        'perf',
        'alert-config-sk-issue-tracker'
      );
    });
    it('does not show email or issue tracker if window.perf.notification is none', async () => {
      await testBed.page.click('#hide_notification');
      await takeScreenshot(
        testBed.page,
        'perf',
        'alert-config-sk-no-notification'
      );
    });
    it('shows an error if the issue tracker component is not a valid int', async () => {
      await testBed.page.click('#invalid_component');
      await takeScreenshot(
        testBed.page,
        'perf',
        'alert-config-sk-invalid-component'
      );
    });
    it('shows alert action options if need_alert_action is true', async () => {
      await testBed.page.click('#show_alert_actions');
      await takeScreenshot(
        testBed.page,
        'perf',
        'alert-config-sk-alert-actions'
      );
    });
  });
});
