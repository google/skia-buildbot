import './index';
import { PerfScaffoldSk } from './perf-scaffold-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import { SkPerfConfig } from '../json';
import fetchMock from 'fetch-mock';

declare const sinon: any;

describe('perf-scaffold-sk', () => {
  const newInstance = setUpElementUnderTest<PerfScaffoldSk>('perf-scaffold-sk');

  beforeEach(() => {
    // Default window.perf to something safe.
    window.perf = {
      instance_url: '',
      instance_name: 'chrome-perf-test',
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
      enable_v2_ui: false,
      dev_mode: false,
    } as unknown as SkPerfConfig;
    window.localStorage.clear();
  });

  afterEach(() => {
    fetchMock.reset();
  });

  it('renders with default logo when header_image_url is empty', async () => {
    window.perf.header_image_url = '';
    const element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    const img = element.querySelector('.logo') as HTMLImageElement;
    assert.include(img.src, 'alpine_transparent.png');
  });

  it('renders with configured logo when header_image_url is set', async () => {
    window.perf.header_image_url = 'http://example.com/logo.png';
    const element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    const img = element.querySelector('.logo') as HTMLImageElement;
    assert.equal(img.src, 'http://example.com/logo.png');
  });

  it('falls back to default logo on error', async () => {
    window.perf.header_image_url = 'http://example.com/invalid.png';
    const element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    const img = element.querySelector('.logo') as HTMLImageElement;

    // Simulate error
    img.dispatchEvent(new Event('error'));

    assert.include(img.src, 'alpine_transparent.png');
  });

  it('V2 UI: renders with default logo when header_image_url is empty', async () => {
    window.perf.header_image_url = '';
    const element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    const img = element.querySelector('.logo') as HTMLImageElement;
    assert.include(img.src, 'alpine_transparent.png');
  });

  it('V2 UI: falls back to default logo on error', async () => {
    window.perf.header_image_url = 'http://example.com/invalid.png';
    const element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    const img = element.querySelector('.logo') as HTMLImageElement;

    // Simulate error
    img.dispatchEvent(new Event('error'));

    assert.include(img.src, 'alpine_transparent.png');
  });

  it('displays instance_name from config', async () => {
    window.perf.instance_name = 'Test Instance';
    const element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'Test Instance');
  });

  it('falls back to extracting name from URL if instance_name is empty', async () => {
    window.perf.instance_name = '';
    window.perf.instance_url = 'https://foo.perf.skia.org';
    const element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'Foo');
  });

  it('truncates long instance names', async () => {
    const longName = 'A'.repeat(70);
    window.perf.instance_name = longName;
    const element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'A'.repeat(64));
  });

  it('V2 UI: displays instance_name from config', async () => {
    window.perf.instance_name = 'Test Instance V2';
    const element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'Test Instance V2');
  });

  it('V2 UI: falls back to extracting name from URL if instance_name is empty', async () => {
    window.perf.instance_name = '';
    window.perf.instance_url = 'https://bar.perf.skia.org';
    const element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'Bar');
  });

  describe('auto-refresh', () => {
    let clock: any;

    afterEach(() => {
      if (clock) {
        clock.restore();
        clock = null;
      }
    });

    it('does not poll in prod mode', async () => {
      window.perf.dev_mode = false;
      fetchMock.get('/_/dev/version', { version: 123 });

      clock = sinon.useFakeTimers();
      newInstance();

      // Fast forward time to trigger interval
      clock.tick(2010);
      assert.isFalse(fetchMock.called('/_/dev/version'));
    });

    it('starts polling in dev mode', async () => {
      window.perf.dev_mode = true;
      fetchMock.get('/_/dev/version', { version: 123 });

      clock = sinon.useFakeTimers();
      newInstance();

      // Fast forward time to trigger interval
      clock.tick(2010);
      assert.isTrue(fetchMock.called('/_/dev/version'));
    });
  });
});
