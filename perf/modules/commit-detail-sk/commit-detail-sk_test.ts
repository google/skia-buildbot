import './index';
import { assert } from 'chai';
import { CommitDetailSk } from './commit-detail-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Commit, CommitNumber } from '../json';

describe('commit-detail-sk', () => {
  const newInstance = setUpElementUnderTest<CommitDetailSk>('commit-detail-sk');

  let element: CommitDetailSk;
  beforeEach(() => {
    element = newInstance();
  });

  const commit: Commit = {
    author: 'alice@example.com',
    message: 'Fixed a bug',
    url: 'https://skia.googlesource.com/infra/+/12345678',
    ts: 1600000000,
    hash: '1234567890abcdef',
    offset: CommitNumber(100),
    body: 'This is a long body message.',
  };

  it('renders commit details', () => {
    element.cid = commit;
    const pre = element.querySelector('pre')!;
    assert.include(pre.textContent, '12345678');
    assert.include(pre.textContent, 'alice@example.com');
    assert.include(pre.textContent, 'Fixed a bug');
  });

  it('renders links correctly', async () => {
    element.cid = commit;
    const buttons = element.querySelectorAll<HTMLElement>('md-outlined-button');
    assert.equal(buttons.length, 4);

    const openedUrls: string[] = [];
    window.open = (url?: string | URL, _target?: string, _features?: string): Window | null => {
      openedUrls.push(url as string);
      return null;
    };

    for (const button of Array.from(buttons)) {
      button.click();
      await Promise.resolve();
    }

    assert.deepEqual(openedUrls, [
      `/g/e/${commit.hash}`,
      `/g/c/${commit.hash}`,
      `/g/t/${commit.hash}`,
      commit.url,
    ]);
  });
});
