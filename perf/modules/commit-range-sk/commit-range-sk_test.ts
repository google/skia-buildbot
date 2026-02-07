import './index';
import { assert } from 'chai';
import * as sinon from 'sinon';
import { CommitRangeSk } from './commit-range-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CommitNumber, TimestampSeconds } from '../json';
import { MISSING_DATA_SENTINEL } from '../const/const';

describe('commit-range-sk', () => {
  const newInstance = setUpElementUnderTest<CommitRangeSk>('commit-range-sk');

  let element: CommitRangeSk;
  beforeEach(() => {
    window.perf = {
      dev_mode: false,
      instance_url: '',
      instance_name: 'chrome-perf-test',
      header_image_url: '',
      commit_range_url: 'http://example.com/range/+log/{begin}..{end}',
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
      extra_links: null,
    };

    element = newInstance((el: CommitRangeSk) => {
      el.commitIndex = 2;
      el.header = [
        {
          offset: CommitNumber(64809),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: 'h64809',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(64810),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: 'h64810',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(64811),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: 'h64811',
          message: '',
          url: '',
        },
      ];
    });
  });

  afterEach(() => {
    sinon.restore();
  });

  describe('converts commit ids to hashes', () => {
    it('ignores MISSING_DATA_SENTINEL', async () => {
      // eslint-disable-next-line dot-notation
      element['commitNumberToHashes'] = async (cids: CommitNumber[]) => {
        assert.deepEqual(cids, [64809, 64811]);
        return ['11111111111111111111111111111', '33333333333333333333333333333'];
      };
      // The MISSING_DATA_SENTINEL should be skipped.
      element.trace = [12, MISSING_DATA_SENTINEL, 13];
      await element.recalcLink();
      assert.equal(
        element.querySelector<HTMLAnchorElement>('a')!.href,
        'http://example.com/range/+log/11111111111111111111111111111..33333333333333333333333333333'
      );
    });
  });

  it('returns the previous hash if there are no missing commits', async () => {
    // eslint-disable-next-line dot-notation
    element['commitNumberToHashes'] = async (cids: CommitNumber[]) => {
      // There were no commits to skip, so return the two consecutive hashes.
      assert.deepEqual(cids, [64810, 64811]);
      return ['11111111111111111111111111111', '22222222222222222222222222222'];
    };
    element.trace = [11, 12, 13];
    await element.recalcLink();
    assert.equal(
      element.querySelector<HTMLAnchorElement>('a')!.href,
      'http://example.com/range/+/22222222222222222222222222222'
    );
  });

  it('handles GitHub commit URL format', async () => {
    window.perf.commit_range_url = 'https://github.com/example/repo/commits/{end}';
    // eslint-disable-next-line dot-notation
    element['commitNumberToHashes'] = async (cids: CommitNumber[]) => {
      assert.deepEqual(cids, [64810, 64811]);
      return ['11111111111111111111111111111', '22222222222222222222222222222'];
    };
    element.trace = [11, 12, 13];
    await element.recalcLink();
    assert.equal(
      element.querySelector<HTMLAnchorElement>('a')!.href,
      'https://github.com/example/repo/commits/22222222222222222222222222222'
    );
  });

  describe('placeholder link, concurrency and caching', () => {
    it('sets a placeholder link while loading', async () => {
      let resolveHashes: (value: string[]) => void;
      const hashesPromise = new Promise<string[]>((resolve) => {
        resolveHashes = resolve;
      });
      // eslint-disable-next-line dot-notation
      element['commitNumberToHashes'] = () => hashesPromise;

      element.trace = [11, 12, 13];
      const recalcPromise = element.recalcLink();

      const link = element.querySelector<HTMLAnchorElement>('a')!;
      assert.equal(link.href, 'javascript:void(0)');
      assert.equal(link.style.cursor, 'default');
      assert.equal(link.textContent, '64811');

      resolveHashes!(['1111', '2222']);
      await recalcPromise;

      assert.equal(
        element.querySelector<HTMLAnchorElement>('a')!.href,
        'http://example.com/range/+/2222'
      );
    });

    it('does not update UI with stale requests', async () => {
      let resolveFirst: (value: string[]) => void;
      const firstPromise = new Promise<string[]>((resolve) => {
        resolveFirst = resolve;
      });

      let resolveSecond: (value: string[]) => void;
      const secondPromise = new Promise<string[]>((resolve) => {
        resolveSecond = resolve;
      });

      let callCount = 0;
      // eslint-disable-next-line dot-notation
      element['commitNumberToHashes'] = async () => {
        callCount++;
        if (callCount === 1) {
          return firstPromise;
        }
        return secondPromise;
      };

      element.trace = [11, 12, 13];
      const firstRecalc = element.recalcLink();

      element.commitIndex = 1;
      const secondRecalc = element.recalcLink();

      resolveSecond!(['2222', '3333']);
      await secondRecalc;
      const finalHref = element.querySelector<HTMLAnchorElement>('a')!.href;
      assert.include(finalHref, '3333');

      resolveFirst!(['1111', '2222']);
      await firstRecalc;

      assert.equal(element.querySelector<HTMLAnchorElement>('a')!.href, finalHref);
    });

    it('uses cache for subsequent requests', async () => {
      const fetchStub = sinon.stub(window, 'fetch');
      fetchStub.callsFake(() =>
        Promise.resolve(
          new Response(
            JSON.stringify({
              commitSlice: [{ hash: 'hash-start' }, { hash: 'hash-end' }],
            }),
            {
              status: 200,
              headers: { 'Content-Type': 'application/json' },
            }
          )
        )
      );

      element.trace = [11, 12, 13];
      await element.recalcLink();
      const countAfterFirst = fetchStub.callCount;
      assert.isAtLeast(countAfterFirst, 1);
      assert.include(element.querySelector<HTMLAnchorElement>('a')!.href, 'hash-end');

      await element.recalcLink();
      assert.equal(fetchStub.callCount, countAfterFirst);
    });
  });
});
