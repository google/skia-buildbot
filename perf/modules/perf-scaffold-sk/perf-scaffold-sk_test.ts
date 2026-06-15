import './index';
import { PerfScaffoldSk } from './perf-scaffold-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import { SkPerfConfig } from '../json';
import fetchMock from 'fetch-mock';

declare const sinon: any;

describe('perf-scaffold-sk', () => {
  const newInstance = setUpElementUnderTest<PerfScaffoldSk>('perf-scaffold-sk');

  let element: PerfScaffoldSk;

  beforeEach(() => {
    // Default window.perf to something safe.
    window.perf = {
      header_image_url: '',
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
      build_date: '2026-03-18',
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
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    await element.updateComplete;
    const img = element.querySelector('.logo') as HTMLImageElement;
    assert.include(img.src, 'alpine_transparent.png');
  });

  it('renders with configured logo when header_image_url is set', async () => {
    window.perf.header_image_url = 'http://example.com/logo.png';
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    await element.updateComplete;
    const img = element.querySelector('.logo') as HTMLImageElement;
    assert.equal(img.src, 'http://example.com/logo.png');
  });

  it('falls back to default logo on error', async () => {
    window.perf.header_image_url = 'http://example.com/invalid.png';
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    await element.updateComplete;
    const img = element.querySelector('.logo') as HTMLImageElement;

    // Simulate error
    img.dispatchEvent(new Event('error'));

    assert.include(img.src, 'alpine_transparent.png');
  });

  it('V2 UI: renders with default logo when header_image_url is empty', async () => {
    window.perf.header_image_url = '';
    element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    await element.updateComplete;
    const img = element.querySelector('.logo') as HTMLImageElement;
    assert.include(img.src, 'alpine_transparent.png');
  });

  it('V2 UI: falls back to default logo on error', async () => {
    window.perf.header_image_url = 'http://example.com/invalid.png';
    element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    await element.updateComplete;
    const img = element.querySelector('.logo') as HTMLImageElement;

    // Simulate error
    img.dispatchEvent(new Event('error'));

    assert.include(img.src, 'alpine_transparent.png');
  });

  it('displays instance_name from config', async () => {
    window.perf.instance_name = 'Test Instance';
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    await element.updateComplete;
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'Test Instance');
  });

  it('falls back to extracting name from URL if instance_name is empty', async () => {
    window.perf.instance_name = '';
    window.perf.instance_url = 'https://foo.perf.skia.org';
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    await element.updateComplete;
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'Foo');
  });

  it('truncates long instance names', async () => {
    const longName = 'A'.repeat(70);
    window.perf.instance_name = longName;
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    await element.updateComplete;
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'A'.repeat(64));
  });

  it('V2 UI: displays instance_name from config', async () => {
    window.perf.instance_name = 'Test Instance V2';
    element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    await element.updateComplete;
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'Test Instance V2');
  });

  it('V2 UI: falls back to extracting name from URL if instance_name is empty', async () => {
    window.perf.instance_name = '';
    window.perf.instance_url = 'https://bar.perf.skia.org';
    element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    await element.updateComplete;
    const title = element.querySelector('.name');
    assert.equal(title?.textContent, 'Bar');
  });

  it('renders build date', async () => {
    window.perf.build_date = '2026-03-18';
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    await element.updateComplete;
    const buildDate = element.querySelector('.build-date');
    assert.equal(buildDate?.textContent?.trim(), 'Build: 2026-03-18');
  });

  it('does not render build date for dev-build', async () => {
    window.perf.app_version = 'dev-2025-11-10T21:55:47Z';
    window.perf.build_date = '2025-11-10';
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    const buildDate = element.querySelector('.build-date');
    assert.isNull(buildDate);
  });

  it('renders gemini-side-panel-sk in V2 UI', async () => {
    window.perf.enable_v2_ui = true;
    element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    await element.updateComplete;
    assert.isNotNull(element.querySelector('gemini-side-panel-sk'));
  });

  it('does not render gemini-side-panel-sk in Legacy UI', async () => {
    window.perf.enable_v2_ui = false;
    element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    await element.updateComplete;
    assert.isNull(element.querySelector('gemini-side-panel-sk'));
  });

  it('toggles gemini-side-panel-sk when button is clicked', async () => {
    window.perf.enable_v2_ui = true;
    element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    await element.updateComplete;

    const button = element.querySelector('button[title="Ask Gemini"]') as HTMLButtonElement;
    const panel = element.querySelector('gemini-side-panel-sk') as any;

    assert.isNotNull(button);
    assert.isNotNull(panel);
    assert.isFalse(panel.open);

    button.click();
    assert.isTrue(panel.open);

    button.click();
    assert.isFalse(panel.open);
  });

  it('defaults to legacy UI when enable_v2_ui is false', async () => {
    window.perf.enable_v2_ui = false;
    element = newInstance((_) => {
      localStorage.clear();
    });
    await element.updateComplete;
    assert.isNotNull(element.querySelector('.legacy-ui'));
    assert.isNull(element.querySelector('.v2-ui'));
  });

  it('defaults to V2 UI when enable_v2_ui is true', async () => {
    window.perf.enable_v2_ui = true;
    element = newInstance((_) => {
      localStorage.clear();
    });
    await element.updateComplete;
    assert.isNotNull(element.querySelector('.v2-ui'));
    assert.isNull(element.querySelector('.legacy-ui'));
  });

  it('honors localStorage preference "false" even if enable_v2_ui is true', async () => {
    window.perf.enable_v2_ui = true;
    element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'false');
    });
    await element.updateComplete;
    assert.isNotNull(element.querySelector('.legacy-ui'));
    assert.isNull(element.querySelector('.v2-ui'));
  });

  it('renders V2 UI toggle even when enable_v2_ui is false', async () => {
    window.perf.enable_v2_ui = false;
    element = newInstance((_) => {
      localStorage.clear();
    });
    await element.updateComplete;
    const toggle = element.querySelector('.try-v2-ui');
    assert.isNotNull(toggle);
  });

  it('toggles to V2 UI when "Try V2 UI" is clicked', async () => {
    window.perf.enable_v2_ui = false;
    element = newInstance((_) => {
      localStorage.clear();
    });
    await element.updateComplete;
    // Stub _reload to prevent page reload during test
    (element as any)._reload = sinon.spy();

    const toggle = element.querySelector('.try-v2-ui') as HTMLButtonElement;
    assert.isNotNull(toggle);
    toggle.click();

    assert.equal(localStorage.getItem('v2_ui'), 'true');
    assert.isTrue((element as any)._reload.called);
  });

  it('toggles to Legacy UI when "Back to Legacy UI" is clicked', async () => {
    window.perf.enable_v2_ui = true; // Start with V2 UI
    element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    await element.updateComplete;
    // Stub _reload to prevent page reload during test
    (element as any)._reload = sinon.spy();

    const toggle = element.querySelector('#legacy-ui-button') as HTMLButtonElement;
    assert.isNotNull(toggle);
    toggle.click();

    assert.equal(localStorage.getItem('v2_ui'), 'false');
    assert.isTrue((element as any)._reload.called);
  });

  describe('multigraphUrl and V2 support', () => {
    beforeEach(() => {
      localStorage.removeItem('perf:use-explore-v2');
      window.perf.default_to_explore_v2 = false;
      window.perf.default_to_manual_plot_mode = false;
    });

    afterEach(() => {
      localStorage.removeItem('perf:use-explore-v2');
    });

    it('returns /m by default', () => {
      element = newInstance();
      assert.equal((element as any).multigraphUrl, '/m');
    });

    it('returns /m?manual_plot_mode=true when default_to_manual_plot_mode is true', () => {
      window.perf.default_to_manual_plot_mode = true;
      element = newInstance();
      assert.equal((element as any).multigraphUrl, '/m?manual_plot_mode=true');
    });

    it('returns /e2 when V2 is enabled via localStorage', () => {
      localStorage.setItem('perf:use-explore-v2', 'true');
      element = newInstance();
      assert.equal((element as any).multigraphUrl, '/e2');
    });

    it('returns /m when V2 is disabled via localStorage even if default config is true', () => {
      localStorage.setItem('perf:use-explore-v2', 'false');
      window.perf.default_to_explore_v2 = true;
      element = newInstance();
      assert.equal((element as any).multigraphUrl, '/m');
    });

    it('returns /e2 when V2 is enabled via default config', () => {
      window.perf.default_to_explore_v2 = true;
      element = newInstance();
      assert.equal((element as any).multigraphUrl, '/e2');
    });

    it('ignores manual_plot_mode when V2 is enabled', () => {
      window.perf.default_to_explore_v2 = true;
      window.perf.default_to_manual_plot_mode = true;
      element = newInstance();
      assert.equal((element as any).multigraphUrl, '/e2');
    });
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

    it('checks version in dev mode', async () => {
      window.perf.dev_mode = true;
      fetchMock.get('/_/dev/version', { version: 123 });

      clock = sinon.useFakeTimers();
      newInstance();

      // Ensure it's called once immediately (or microtask soon)
      assert.isTrue(fetchMock.called('/_/dev/version'));

      // Advance time to verify it does NOT poll
      fetchMock.resetHistory();
      clock.tick(2010);
      assert.isFalse(fetchMock.called('/_/dev/version'));
    });
  });
});
