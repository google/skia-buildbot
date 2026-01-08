/* eslint-disable dot-notation */
import './index';
import '../../../elements-sk/modules/error-toast-sk';
import { $$ } from '../../../infra-sk/modules/dom';
import { ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { setUpExploreDemoEnv } from '../common/test-util';

setUpExploreDemoEnv();

window.perf = {
  dev_mode: false,
  instance_url: '',
  instance_name: 'chrome-perf-demo',
  header_image_url: '',
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
};

// TODO(b/450184956) Rewrite demo to plot-google-chart. This demo is unusable.
void customElements.whenDefined('explore-sk').then(() => {
  // Insert the element later, which should give enough time for fetchMock to be in place.
  document
    .querySelector('h1')!
    .insertAdjacentElement('afterend', document.createElement('explore-sk'));

  const explore = $$<ExploreSimpleSk>('explore-simple-sk');
  if (explore) {
    explore.state.enable_chart_tooltip = true;
  }

  // Some utility functions used later.

  // TODO(b/450184956) Rewrite demo to plot-google-chart
  // Clicks inside the canvas element inside the plot-simple-sk element.
  const clickOnPlot = () => {
    const rect = explore!.querySelector<HTMLCanvasElement>('canvas')!.getBoundingClientRect();
    // eslint-disable-next-line dot-notation

    explore!['googleChartPlot'].value!.dispatchEvent(
      new MouseEvent('click', {
        // Pick a point in the middle of the canvas.
        clientX: rect.left + rect.width / 2,
        clientY: rect.top + rect.height / 2,
      })
    );
  };

  // Calls itself via timeout until the plot-simple-sk element reports that it
  // has traces loaded, at which point it calls clickOnPlot.
  // TODO(b/450184956) Rewrite demo to plot-google-chart

  const checkIfLoaded = () => {
    // eslint-disable-next-line dot-notation
    if (explore!['googleChartPlot'].value!.getAllTraces().length > 1) {
      clickOnPlot();
    } else {
      setTimeout(checkIfLoaded, 100);
    }
  };

  // Add handlers for the all demo buttons at the top of the demo page. These
  // buttons are triggered by the puppeteer tests.

  $$('#demo-show-query-dialog')?.addEventListener('click', () => {
    $$<HTMLButtonElement>('#open_query_dialog')!.click();
    $$<HTMLDetailsElement>('#time-range-summary')!.open = true;
  });

  $$('#demo-load-traces')?.addEventListener('click', () => {
    window.perf.fetch_chrome_perf_anomalies = false;
    // eslint-disable-next-line dot-notation
    explore!['query']!.current_query = 'arch=arm';
    explore!.add(true, 'query');
  });

  $$('#demo-show-bisect-button')?.addEventListener('click', () => {
    window.perf.fetch_chrome_perf_anomalies = true;
    // eslint-disable-next-line dot-notation
    explore!['query']!.current_query = 'arch=arm';
    explore!.add(true, 'query');
  });

  $$('#demo-select-trace')?.addEventListener('click', () => {
    window.perf.fetch_chrome_perf_anomalies = true;

    // First load the data.
    explore!['query']!.current_query = 'arch=arm';
    explore!.add(true, 'query');

    // Then wait until the data has loaded before sending the
    // synthetic mouse click.
    setTimeout(checkIfLoaded, 100);
  });

  $$('#demo-select-calc-trace')?.addEventListener('click', () => {
    // First load data.
    explore!['query']!.current_query = 'arch=arm';
    explore!.add(true, 'query');

    // Then wait until the data has loaded before sending the
    // synthetic mouse click.
    setTimeout(checkIfLoaded, 100);
  });

  $$('#demo-show-help')?.addEventListener('click', () => {
    // eslint-disable-next-line dot-notation
    explore!['keyDown'](
      new KeyboardEvent('keydown', {
        isComposing: false,
        key: '?',
      })
    );
  });

  $$('#demo-highlighted-only')?.addEventListener('click', () => {
    // eslint-disable-next-line dot-notation
    explore!['query']!.current_query = 'arch=arm';
    explore!.add(true, 'query');
    $$<HTMLButtonElement>('#highlighted-only')!.click();
  });
});
