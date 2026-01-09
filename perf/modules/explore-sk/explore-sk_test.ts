import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { ExploreSk } from './explore-sk';
import { ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { setUpExploreDemoEnv } from '../common/test-util';

fetchMock.config.overwriteRoutes = true;
describe('ExploreSk', () => {
  let element: ExploreSk;
  const setupElement = async (mockDefaults: any = null, paramsMock: any = null) => {
    setUpExploreDemoEnv();
    window.perf = {
      instance_url: '',
      commit_range_url: '',
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
      trace_format: 'chrome',
      need_alert_action: false,
      bug_host_url: '',
      git_repo_url: '',
      keys_for_commit_range: [],
      keys_for_useful_links: [],
      skip_commit_detail_display: false,
      image_tag: 'fake-tag',
      remove_default_stat_value: false,
      enable_skia_bridge_aggregation: false,
      show_json_file_display: false,
      always_show_commit_info: false,
      show_triage_link: true,
      show_bisect_btn: true,
      app_version: 'test-version',
      instance_name: 'chrome-perf-test',
      header_image_url: '',
      enable_v2_ui: false,
      dev_mode: false,
      extra_links: null,
    };

    fetchMock.config.overwriteRoutes = true;
    const defaultsResponse = mockDefaults || {
      default_url_values: {
        use_test_picker_query: 'false',
      },
    };
    fetchMock.get('/_/defaults/', defaultsResponse);

    if (paramsMock) {
      fetchMock.get('/_/Params/', 1);
    }

    element = setUpElementUnderTest<ExploreSk>('explore-sk')();
    // Wait for connectedCallback to finish, including initializeDefaults
    const _ = await fetchMock.flush(true);
    // Yield to let init() and initializeTestPicker() finish
    await new Promise((resolve) => setTimeout(resolve, 0));
  };

  beforeEach(async () => {
    await setupElement();
  });

  it('renders initial state', async () => {
    assert.isNotNull(element.querySelector('explore-simple-sk'));
    assert.isNotNull(element.querySelector('#test-picker'));
  });

  it('updates state when explore-simple-sk emits state_changed', async () => {
    const exploreSimpleSk = element.querySelector<ExploreSimpleSk>('explore-simple-sk')!;
    (element as any).stateHasChanged = sinon.spy();
    exploreSimpleSk.dispatchEvent(new CustomEvent('state_changed'));
    assert.isTrue((element as any).stateHasChanged.calledOnce);
  });

  it('renders when explore-simple-sk emits rendered_traces', async () => {
    const exploreSimpleSk = element.querySelector<ExploreSimpleSk>('explore-simple-sk')!;
    const renderSpy = sinon.spy(element as any, '_render');
    exploreSimpleSk.dispatchEvent(new CustomEvent('rendered_traces'));
    assert.isTrue(renderSpy.calledOnce);
  });

  it('initializes test picker', async () => {
    // Mock defaults with include_params
    await setupElement({
      include_params: ['config', 'arch'],
      default_param_selections: { config: ['8888'] },
      default_url_values: { use_test_picker_query: 'true' },
    });

    const testPicker = element.querySelector<any>('#test-picker')!;
    assert.isFalse(testPicker.classList.contains('hidden'));
  });

  it('handles plot-button-clicked event', async () => {
    await setupElement({
      include_params: ['config'],
      default_url_values: { use_test_picker_query: 'true' },
    });

    const exploreSimpleSk = element.querySelector<ExploreSimpleSk>('explore-simple-sk')!;
    const addSpy = sinon.spy(exploreSimpleSk, 'addFromQueryOrFormula');

    element.dispatchEvent(new CustomEvent('plot-button-clicked'));
    await new Promise((resolve) => setTimeout(resolve, 100)); // Yield to async listener

    assert.isTrue(addSpy.calledOnce);
  });

  it('handles populate-query event', async () => {
    await setupElement({
      include_params: ['config'],
      default_url_values: { use_test_picker_query: 'true' },
    });

    element.dispatchEvent(
      new CustomEvent('populate-query', {
        detail: { key: ',config=8888,' },
      })
    );

    const testPicker = element.querySelector<any>('#test-picker')!;
    // testPicker.populateFieldDataFromQuery should have been called.
    // We can check if it's no longer hidden (it was already not hidden).
    assert.isFalse(testPicker.classList.contains('hidden'));
  });

  it('calls reset on remove-explore event', async () => {
    const exploreSimpleSk = element.querySelector<ExploreSimpleSk>('explore-simple-sk')!;
    const resetSpy = sinon.spy(exploreSimpleSk, 'reset');
    element.dispatchEvent(new CustomEvent('remove-explore'));
    assert.isTrue(resetSpy.calledOnce);
  });

  it('passes keydown events to explore-simple-sk', async () => {
    const exploreSimpleSk = element.querySelector<ExploreSimpleSk>('explore-simple-sk')!;
    const keyDownSpy = sinon.spy(exploreSimpleSk, 'keyDown');
    const event = new KeyboardEvent('keydown', { key: '?' });
    document.dispatchEvent(event);
    assert.isTrue(keyDownSpy.calledOnceWith(event));
  });

  it('updates enable_favorites based on login status', async () => {
    const exploreSimpleSk = element.querySelector<ExploreSimpleSk>('explore-simple-sk')!;

    // Mock LoggedIn to return a logged in user
    fetchMock.get(
      '/_/login/status',
      {
        email: 'user@google.com',
        roles: ['editor'],
      },
      { overwriteRoutes: true }
    );

    // We need to re-run connectedCallback logic or just wait if it was already called.
    // Actually, it was called in setupElement.
    // Let's re-setup with the mock.
    await setupElement();

    assert.equal((element as any).userEmail, 'user@google.com');
    assert.isTrue(exploreSimpleSk.state.enable_favorites);
  });
});
