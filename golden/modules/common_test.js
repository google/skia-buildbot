import { $$ } from 'common-sk/modules/dom';
import {
  humanReadableQuery,
  digestImagePath,
  digestDiffImagePath,
  detailHref,
  diffPageHref,
  truncateWithEllipses,
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

describe('shorten', () => {
  it('shortens long strings into possibly ellipsed versions', () => {
    expect(truncateWithEllipses('')).to.equal('');
    expect(truncateWithEllipses('too short')).to.equal('too short');
    expect(truncateWithEllipses('should be ellipsed because it is too long')).to.equal('should be el...');
    expect(truncateWithEllipses('should be ellipsed because it is too long', 20)).to.equal('should be ellipse...');
    expect(truncateWithEllipses('should be ellipsed because it is too long', 5)).to.equal('sh...');
  });
  it('throws an exception if maxLength is too short', () => {
    try {
      truncateWithEllipses('foo bar', 2);
    } catch (e) {
      expect(e).to.contain('length of the ellipsis');
      return;
    }
    expect.fail('There should have been an exception.');
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
  it('returns a path with and without an issue', () => {
    expect(detailHref('my-test', aDigest)).to.equal(
      '/detail?test=my-test&digest=aaab78c9711cb79197d47f448ba51338',
    );
    expect(detailHref('my-test', aDigest, '12345')).to.equal(
      '/detail?test=my-test&digest=aaab78c9711cb79197d47f448ba51338&issue=12345',
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
    expect(diffPageHref('my-test', aDigest, bDigest, '12345')).to.equal(
      '/diff?test=my-test&left=aaab78c9711cb79197d47f448ba51338&right=bbb8b07beb4e1247c2cbafdb92b93e55&issue=12345',
    );
  });
});

describe('event functions', () => {
  let ele;
  beforeEach(() => {
    ele = document.createElement('div');
    $$('body').appendChild(ele);
  });

  afterEach(() => {
    $$('body').removeChild(ele);
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
    const evt = eventPromise('fetch-error');
    sendFetchError(ele, 'some error', 'loading stuff');
    const e = await evt;
    expect(e.detail.error).to.equal('some error');
    expect(e.detail.loading).to.equal('loading stuff');
  });
});
