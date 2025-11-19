import './user-issue-sk';
import { expect } from 'chai';
import { UserIssueSk } from './user-issue-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import fetchMock from 'fetch-mock';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import { render } from 'lit-html';

describe('user-issue-sk', () => {
  const newInstance = setUpElementUnderTest<UserIssueSk>('user-issue-sk');

  let element: UserIssueSk;
  beforeEach(() => {
    element = newInstance();
    window.perf = {
      instance_url: '',
      instance_name: 'chrome-perf-test',
      header_image_url: '',
      commit_range_url: 'http://example.com/range/{begin}/{end}',
      key_order: ['config'],
      demo: true,
      radius: 7,
      num_shift: 10,
      interesting: 25,
      step_up_only: false,
      display_group_by: true,
      hide_list_of_commits_on_explore: false,
      notifications: 'none',
      fetch_chrome_perf_anomalies: false,
      fetch_anomalies_from_sql: false,
      feedback_url: '',
      chat_url: '',
      help_url_override: '',
      trace_format: '',
      need_alert_action: false,
      bug_host_url: 'http://test.com/p/skia/issues/detail',
      git_repo_url: '',
      keys_for_commit_range: [],
      keys_for_useful_links: [],
      skip_commit_detail_display: false,
      image_tag: 'fake-tag',
      remove_default_stat_value: false,
      enable_skia_bridge_aggregation: false,
      show_json_file_display: false,
      always_show_commit_info: false,
      show_triage_link: false,
      show_bisect_btn: false,
      app_version: 'test-version',
      enable_v2_ui: false,
    };
  });

  afterEach(() => {
    fetchMock.restore();
  });

  describe('render', () => {
    it('renders nothing when bug_id is -1', async () => {
      element.bug_id = -1;
      await element.updateComplete;
      expect(element.shadowRoot!.childElementCount).to.equal(0);
    });

    it('renders nothing when not logged in and no issue exists', async () => {
      element.bug_id = 0;
      element.user_id = '';
      await element.updateComplete;
      expect(element.shadowRoot!.childElementCount).to.equal(0);
    });

    it('renders add issue button when logged in and no issue exists', async () => {
      element.bug_id = 0;
      element.user_id = 'test@example.com';
      await element.updateComplete;
      const addIssue = element.shadowRoot!.querySelector('#add-issue-button') as HTMLElement;
      expect(addIssue).to.not.equal(null);
    });

    it('renders bug link with delete when logged in and issue exists', async () => {
      element.user_id = 'test@example.com';
      element.issueExists = true;
      await element.updateComplete;
      const showLinkContainer = element.shadowRoot!.querySelector('.showLinkContainer');
      expect(showLinkContainer).to.not.equal(null);

      element.bug_id = 12345;
      await element.updateComplete;
      const bugLink = element.shadowRoot!.querySelector('a');
      expect(bugLink).to.not.equal(null);
      const deleteIcon = element.shadowRoot!.querySelector('close-icon-sk');
      expect(deleteIcon).to.not.equal(null);
    });

    it('renders bug link without delete when not logged in and issue exists', async () => {
      element.bug_id = 12345;
      element.user_id = '';
      element.issueExists = true;
      await element.updateComplete;
      const bugLink = element.shadowRoot!.querySelector('a');
      expect(bugLink).to.equal(null);
      const deleteIcon = element.shadowRoot!.querySelector('close-icon-sk');
      expect(deleteIcon).to.equal(null);
    });
  });

  describe('addOrFindIssue', () => {
    it('finds an existing issue and displays the link', async () => {
      element.user_id = 'test@example.com';
      element.issueExists = false;
      await element.updateComplete;
      const addIssueBtn = element.shadowRoot!.querySelector(
        '#add-issue-button'
      ) as HTMLButtonElement;
      addIssueBtn.click();
      element._input_val = 12345;
      await element.updateComplete;

      const checkIcon = element.shadowRoot!.querySelector('#check-icon') as HTMLButtonElement;
      fetchMock.post('/_/user_issues', { UserIssues: [{ IssueId: 12345 }] });
      fetchMock.post('/_/user_issue/save', {});
      checkIcon.click();
      await fetchMock.flush(true);

      expect(element.bug_id).to.equal(12345);
      const bugLink = element.shadowRoot!.querySelector('a');
      expect(bugLink).to.not.equal(null);
      expect(bugLink!.href).to.equal('http://test.com/p/skia/issues/detail/12345');
    });

    it('If the bug not exists, files a new bug and displays the link', async () => {
      element.user_id = 'test@example.com';
      element.bug_id = 0;

      await element.updateComplete;
      const addIssueBtn = element.shadowRoot!.querySelector(
        '#add-issue-button'
      ) as HTMLButtonElement;
      addIssueBtn.click();
      expect(element._text_input_active).to.equal(true);

      element._input_val = 12345;
      await element.updateComplete;
      const checkIcon = element.shadowRoot!.querySelector('#check-icon') as HTMLButtonElement;
      expect(checkIcon).to.not.equal(null);

      fetchMock.post('/_/user_issues', { UserIssues: [{ IssueId: 11111 }] });
      fetchMock.post('/_/triage/file_bug', { bug_id: 54321 });
      fetchMock.post('/_/user_issue/save', {});
      checkIcon.click();
      await fetchMock.flush(true);
      expect(element.bug_id).to.equal(54321);

      const bugLink = element.shadowRoot!.querySelector('a');
      expect(bugLink).to.not.equal(null);
      expect(bugLink).to.not.equal(null);
    });
  });

  describe('removeIssue', () => {
    it('is called when the delete icon is clicked', async () => {
      fetchMock.post('/_/user_issue/delete', {});
      element.bug_id = 12345;
      element.user_id = 'test@example.com';
      element.issueExists = true;
      await element.updateComplete;

      const deleteIcon = element.shadowRoot!.querySelector('close-icon-sk') as HTMLElement;
      deleteIcon.click();

      await fetchMock.flush(true);
      expect(element.bug_id).to.equal(0);
      expect(element.issueExists).to.equal(false);

      await element.updateComplete;
      const addIssue = element.shadowRoot!.querySelector('#add-issue-button') as HTMLElement;
      expect(addIssue).to.not.equal(null);
    });
  });

  describe('showLinkTemplate', () => {
    it('formats bug correctly', () => {
      const bugId = 12345;
      const formattedBug = AnomalySk.formatBug(window.perf.bug_host_url, bugId);

      const div = document.createElement('div');
      render(formattedBug, div);
      const a = div.querySelector('a')!;
      expect(a).to.not.equal(null);
      expect(a.href).to.equal('http://test.com/p/skia/issues/detail/12345');
      expect(a.target).to.equal('_blank');
      expect(a.textContent).to.equal('12345');
    });
  });
});
