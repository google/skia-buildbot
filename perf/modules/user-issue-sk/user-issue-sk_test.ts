import './user-issue-sk';
import { expect } from 'chai';
import { UserIssueSk } from './user-issue-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import fetchMock from 'fetch-mock';
import { formatBug } from '../common/anomaly';
import { render } from 'lit-html';
import sinon from 'sinon';

describe('user-issue-sk', () => {
  const newInstance = setUpElementUnderTest<UserIssueSk>('user-issue-sk');

  let element: UserIssueSk;
  beforeEach(() => {
    element = newInstance();
    window.perf = {
      dev_mode: false,
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
      extra_links: null,
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

    it('renders add issue buttons when logged in and no issue exists', async () => {
      element.bug_id = 0;
      element.user_id = 'test@example.com';
      await element.updateComplete;
      const buttons = element.shadowRoot!.querySelectorAll('.add-issue');
      expect(buttons.length).to.equal(2);
      expect(buttons[0].textContent).to.equal('Add Existing Bug');
      expect(buttons[1].textContent).to.equal('Add New Bug');
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

  describe('saveissue', () => {
    it('finds an existing issue and displays the link', async () => {
      element.user_id = 'test@example.com';
      element.issueExists = false;
      await element.updateComplete;
      const addExistingIssueBtn = element.shadowRoot!.querySelector(
        '.add-issue'
      ) as HTMLButtonElement;
      addExistingIssueBtn.click();
      element._input_val = 12345;
      await element.updateComplete;

      const checkIcon = element.shadowRoot!.querySelector('#check-icon') as HTMLButtonElement;
      fetchMock.post('/_/user_issue/save', {});
      checkIcon.click();
      await fetchMock.flush(true);

      expect(element.bug_id).to.equal(12345);
      const bugLink = element.shadowRoot!.querySelector('a');
      expect(bugLink).to.not.equal(null);
      expect(bugLink!.href).to.equal('http://test.com/p/skia/issues/detail/12345');
    });
  });

  describe('createNewBug', () => {
    it('shows loading popup, calls create endpoint and redirects', async () => {
      element.user_id = 'test@example.com';
      element.bug_id = 0;
      element.trace_key = 'test-trace';
      element.commit_position = 100;
      await element.updateComplete;

      const addNewBugBtn = element.shadowRoot!.querySelectorAll(
        '.add-issue'
      )[1] as HTMLButtonElement;

      fetchMock.post('/_/user_issue/create', (_, opts) => {
        const body = JSON.parse(opts.body as string);
        expect(body.trace_key).to.equal('test-trace');
        expect(body.commit_position).to.equal(100);
        expect(body.assignee).to.equal('test@example.com');
        return { bug_id: 67890 };
      });

      const windowOpenStub = sinon.stub(window, 'open');

      addNewBugBtn.click();

      await fetchMock.flush(true);

      expect(element.bug_id).to.equal(67890);
      expect(element.issueExists).to.be.true;
      expect(windowOpenStub.calledWith('https://issues.chromium.org/issues/67890', '_blank')).to.be
        .true;
      windowOpenStub.restore();
    });

    it('dispatches user-issue-changed event on success', async () => {
      element.user_id = 'test@example.com';
      element.trace_key = 'test-trace';
      element.commit_position = 100;
      await element.updateComplete;

      fetchMock.post('/_/user_issue/create', { bug_id: 67890 });
      sinon.stub(window, 'open');

      const eventPromise = new Promise<CustomEvent>((resolve) => {
        element.addEventListener('user-issue-changed', (e) => {
          resolve(e as CustomEvent);
        });
      });

      const addNewBugBtn = element.shadowRoot!.querySelectorAll(
        '.add-issue'
      )[1] as HTMLButtonElement;
      addNewBugBtn.click();

      const event = await eventPromise;
      expect(event.detail.bug_id).to.equal(67890);
      expect(event.detail.trace_key).to.equal('test-trace');
      expect(event.detail.commit_position).to.equal(100);

      (window.open as sinon.SinonStub).restore();
    });

    it('handles errors during creation', async () => {
      element.user_id = 'test@example.com';
      await element.updateComplete;

      fetchMock.post('/_/user_issue/create', 500);

      const addNewBugBtn = element.shadowRoot!.querySelectorAll(
        '.add-issue'
      )[1] as HTMLButtonElement;

      // We expect the error to be thrown and an error message to be shown.
      const dialog = element.shadowRoot!.querySelector('#loading-popup') as HTMLDialogElement;
      const closeSpy = sinon.spy(dialog, 'close');

      addNewBugBtn.click();
      await fetchMock.flush(true);

      expect(closeSpy.called).to.be.true;
      expect(element.issueExists).to.be.false;
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
      const buttons = element.shadowRoot!.querySelectorAll('.add-issue');
      expect(buttons.length).to.be.greaterThan(0);
    });
  });

  describe('showLinkTemplate', () => {
    it('formats bug correctly', () => {
      const bugId = 12345;
      const formattedBug = formatBug(window.perf.bug_host_url, bugId);

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
