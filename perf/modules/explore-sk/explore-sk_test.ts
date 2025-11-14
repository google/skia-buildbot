import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { ExploreSk } from './explore-sk';
import { ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { setUpExploreDemoEnv } from '../common/test-util';

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
    await fetchMock.flush(true);
  };

  beforeEach(async () => {
    await setupElement();
  });

  it('calls reset on remove-explore event', async () => {
    const exploreSimpleSk = element.querySelector<ExploreSimpleSk>('explore-simple-sk')!;
    const resetSpy = sinon.spy(exploreSimpleSk, 'reset');
    element.dispatchEvent(new CustomEvent('remove-explore'));
    assert.isTrue(resetSpy.calledOnce);
  });

  it('check openQueryByDefault property based on use_test_picker_query value', async () => {
    const exploreSimpleSk = element.querySelector<ExploreSimpleSk>('explore-simple-sk')!;
    // Based on the logic where openQueryByDefault = !use_test_picker_query
    assert.isFalse(exploreSimpleSk.state.use_test_picker_query);
    assert.isTrue(exploreSimpleSk.openQueryByDefault);
  });
});
