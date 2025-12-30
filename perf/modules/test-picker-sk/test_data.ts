export const BENCHMARK = 'blink_perf.css';
export const BOT = 'ToTLinuxTSan';
export const TEST =
  'memory:chrome:renderer_processes:reported_by_chrome:v8:heap:code_space:effective_size_max';
export const SUBTEST_1 = 'link_invalidation_document_rules.html';
export const SUBTEST_2 = 'AdsAdSenseAsyncAds_warm';

export const TEST_NEW = 'motion_mark_canvas_fill_shapes';
export const SUBTEST_1_NEW = 'line-layout.html';
export const SUBTEST_2_NEW = 'AdsAdSenseAsyncAds_cold';
export const PARAMS = ['benchmark', 'bot', 'test', 'subtest1', 'subtest2'];
export const paramset = {
  benchmark: [
    'ad_frames.fencedframe',
    'base_perftests',
    'blink_perf.accessibility',
    'blink_perf.bindings',
    'blink_perf.css',
    'blink_perf.display_locking',
    'blink_perf.dom',
    'jetstream2',
    'power.desktop',
    'speedometer2',
    'webrtc',
  ],
  bot: [
    'ToTAndroid64',
    'ToTAndroidOfficial',
    'ToTLinux',
    'ToTLinux (dbg)',
    'ToTLinuxMSan',
    'ToTLinuxOfficial',
    'ToTLinuxTSan',
    'ToTLinuxUBSanVptr',
    'ToTMacOfficial',
    'ToTWin',
    'linux-perf',
  ],
  test: [
    'memory:chrome:renderer_processes:reported_by_chrome:v8:heap:code_space:effective_size_max',
    'memory:chrome:renderer_processes:reported_by_chrome:v8:heap:code_space:effective_size_min',
    'motion_mark_canvas_fill_shapes',
    'motion_mark_canvas_stroke_shapes',
    'motionmark_ramp_canvas_arcs',
    'set-attribute.html',
    'sfgate_mobile_2018',
    'shadow-style-share-attr-selectors.html',
    'webui_tab_strip_top10_2020',
    'webui_tab_strip_top10_loading_2020',
    'wikipedia_2018',
    'wikipedia_delayed_scroll_start_2018',
  ],
  subtest1: [
    'line-layout-line-height.html',
    'line-layout-repeat-append-select.html',
    'line-layout-repeat-append.html',
    'line-layout.html',
    'link_invalidation_document_rules.html',
    'link_invalidation_document_rules_sparse.html',
  ],
  subtest2: ['AdsAdSenseAsyncAds_cold', 'AdsAdSenseAsyncAds_hot', 'AdsAdSenseAsyncAds_warm'],
};
