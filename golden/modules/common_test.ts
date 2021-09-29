import { $$ } from 'common-sk/modules/dom';
import { expect } from 'chai';
import {
  humanReadableQuery,
  digestImagePath,
  digestDiffImagePath,
  detailHref,
  diffPageHref,
  sendBeginTask,
  sendEndTask,
  sendFetchError,
} from './common';
import { eventPromise } from '../../infra-sk/modules/test_util';

describe('humanReadableQuery', () => {
  it('turns url encoded queries into human readable version', () => {
    expect(humanReadableQuery('alpha=beta&gamma=delta')).to.equal(
      'alpha=beta\ngamma=delta',
    );
    const inputWithSpaces = "mind%20the%20gap=tube&woody=There's%20a%20space%20in%20my%20boot";
    expect(humanReadableQuery(inputWithSpaces)).to.equal(
      "mind the gap=tube\nwoody=There's a space in my boot",
    );
  });
});

// valid, but arbitrary md5 hashes.
const aDigest = 'aaab78c9711cb79197d47f448ba51338';
const bDigest = 'bbb8b07beb4e1247c2cbafdb92b93e55';

describe('digestImagePath', () => {
  it('returns links to PNGs for a given digest', () => {
    expect(digestImagePath(aDigest)).to.equal('/img/images/aaab78c9711cb79197d47f448ba51338.png');
    expect(digestImagePath(bDigest)).to.equal('/img/images/bbb8b07beb4e1247c2cbafdb92b93e55.png');
  });
});

describe('digestDiffImagePath', () => {
  it('returns the same path no matter the order of the arguments', () => {
    expect(digestDiffImagePath(aDigest, bDigest)).to.equal(
      '/img/diffs/aaab78c9711cb79197d47f448ba51338-bbb8b07beb4e1247c2cbafdb92b93e55.png',
    );
    expect(digestDiffImagePath(bDigest, aDigest)).to.equal(
      '/img/diffs/aaab78c9711cb79197d47f448ba51338-bbb8b07beb4e1247c2cbafdb92b93e55.png',
    );
  });
});

describe('detailHref', () => {
  it('returns a path with and without an changelist id', () => {
    expect(detailHref('my-test', aDigest)).to.equal(
      '/detail?test=my-test&digest=aaab78c9711cb79197d47f448ba51338',
    );
    expect(detailHref('my-test', aDigest, '12345', 'gerrit')).to.equal(
      '/detail?test=my-test&digest=aaab78c9711cb79197d47f448ba51338&changelist_id=12345&crs=gerrit',
    );
  });
});

describe('diffPageHref', () => {
  it('returns a path with the digests in the expected order', () => {
    expect(diffPageHref('my-test', aDigest, bDigest)).to.equal(
      '/diff?test=my-test&left=aaab78c9711cb79197d47f448ba51338&right=bbb8b07beb4e1247c2cbafdb92b93e55',
    );
    // order matters
    expect(diffPageHref('my-test', bDigest, aDigest)).to.equal(
      '/diff?test=my-test&left=bbb8b07beb4e1247c2cbafdb92b93e55&right=aaab78c9711cb79197d47f448ba51338',
    );
  });
  it('supports an optional changelist id', () => {
    expect(diffPageHref('my-test', aDigest, bDigest, '123456', 'github')).to.equal(
      '/diff?test=my-test&left=aaab78c9711cb79197d47f448ba51338'
      + '&right=bbb8b07beb4e1247c2cbafdb92b93e55&changelist_id=123456&crs=github',
    );
  });
});

describe('event functions', () => {
  let ele: Element;
  beforeEach(() => {
    ele = document.createElement('div');
    $$('body')!.appendChild(ele);
  });

  afterEach(() => {
    $$('body')!.removeChild(ele);
  });

  it('sends a begin-task', async () => {
    const evt = eventPromise('begin-task');
    sendBeginTask(ele);
    await evt;
  });

  it('sends a end-task', async () => {
    const evt = eventPromise('end-task');
    sendEndTask(ele);
    await evt;
  });

  it('sends a fetch-error', async () => {
    const evt = eventPromise<CustomEvent>('fetch-error');
    sendFetchError(ele, 'some error', 'loading stuff');
    const e = await evt;
    expect(e.detail.error).to.equal('some error');
    expect(e.detail.loading).to.equal('loading stuff');
  });
});
