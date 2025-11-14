import './index';
import { PerfScaffoldSk } from './perf-scaffold-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';

describe('perf-scaffold-sk', () => {
  const newInstance = setUpElementUnderTest<PerfScaffoldSk>('perf-scaffold-sk');

  beforeEach(() => {
    // Reset window.perf to default values
    window.perf = {
      commit_range_url: '',
      key_order: [],
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
      trace_format: '',
      need_alert_action: false,
      bug_host_url: '',
      git_repo_url: '',
      keys_for_commit_range: [],
      keys_for_useful_links: [],
      skip_commit_detail_display: false,
      image_tag: 'fake-tag',
      header_image_url: '',
      instance_url: 'https://perf.skia.org',
      instance_name: 'chrome-perf-test',
    } as any;
  });

  it('renders with default logo when header_image_url is empty', async () => {
    window.perf.header_image_url = '';
    const element = newInstance((_) => {
      localStorage.removeItem('v2_ui');
    });
    const img = element.querySelector('.logo') as HTMLImageElement;
    assert.include(img.src, 'chrome-logo.svg');
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

    assert.include(img.src, 'chrome-logo.svg');
  });

  it('V2 UI: renders with default logo when header_image_url is empty', async () => {
    window.perf.header_image_url = '';
    const element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    const img = element.querySelector('.logo') as HTMLImageElement;
    assert.include(img.src, 'chrome-logo.svg');
  });

  it('V2 UI: falls back to default logo on error', async () => {
    window.perf.header_image_url = 'http://example.com/invalid.png';
    const element = newInstance((_) => {
      localStorage.setItem('v2_ui', 'true');
    });
    const img = element.querySelector('.logo') as HTMLImageElement;

    // Simulate error
    img.dispatchEvent(new Event('error'));

    assert.include(img.src, 'chrome-logo.svg');
  });
});
