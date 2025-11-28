import './index';
import fetchMock from 'fetch-mock';
import { TestPickerSk } from './test-picker-sk';
import { NextParamListHandlerRequest } from '../json';

import { $$ } from '../../../infra-sk/modules/dom';
import { fromParamSet, toParamSet } from '../../../infra-sk/modules/query';

function delay(time: number) {
  return new Promise((resolve) => setTimeout(resolve, time));
}

const params = ['benchmark', 'bot', 'test', 'subtest1', 'subtest2'];

const paramset = {
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

window.customElements.whenDefined('test-picker-sk').then(() => {
  document.querySelectorAll<TestPickerSk>('test-picker-sk').forEach((ele) => {
    ele.initializeTestPicker(params, {}, false);
    ele.addEventListener('plot-button-clicked', (e) => {
      const log = $$<HTMLPreElement>('#events')!;
      log.textContent = (e as CustomEvent).detail.query;
    });
  });
});

$$('#populate-query')?.addEventListener('click', () => {
  const ele = document.querySelector('test-picker-sk') as TestPickerSk;
  const query = {
    benchmark: ['blink_perf.css'],
    bot: ['ToTLinuxTSan'],
    test: [
      'memory:chrome:renderer_processes:reported_by_chrome:v8:heap:code_space:effective_size_max',
    ],
    subtest1: ['link_invalidation_document_rules.html'],
    subtest2: ['AdsDoubleClickAsyncAds_cold'],
  };
  ele.populateFieldDataFromQuery(fromParamSet(query), params, {});
});

$$('#populate-partial-query')?.addEventListener('click', () => {
  const ele = document.querySelector('test-picker-sk') as TestPickerSk;
  const query = {
    benchmark: ['blink_perf.css'],
    bot: ['ToTLinuxTSan'],
    test: [
      'memory:chrome:renderer_processes:reported_by_chrome:v8:heap:code_space:effective_size_max',
    ],
    subtest1: [''],
    subtest2: ['AdsAdSenseAsyncAds_warm'],
  };
  ele.populateFieldDataFromQuery(fromParamSet(query), params, {});
});

fetchMock.post('/_/nextParamList/', async (_, opts) => {
  const request = JSON.parse(opts.body!.toString()) as NextParamListHandlerRequest;
  const currentParamSet = toParamSet(request.q);
  let nextParam = '';

  // Find the first param from our list that isn't in the current query.
  for (const param of params) {
    if (!(param in currentParamSet)) {
      nextParam = param;
      break;
    }
  }

  const responseParamSet: any = {};
  if (nextParam && nextParam in paramset) {
    // Return only the next param and its options.
    responseParamSet[nextParam] = (paramset as any)[nextParam];
  }

  // Calculate a mock count based on how many fields are left to fill.
  let emptyValues = 0;
  params.forEach((param) => {
    if (!(param in currentParamSet)) {
      emptyValues += 1;
    }
  });

  await delay(100); // Reduced delay for faster tests
  return { paramset: responseParamSet, count: emptyValues * 5 + 1 };
});
