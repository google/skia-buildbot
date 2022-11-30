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
  clusterPageHref,
} from './common';
import { eventPromise } from '../../infra-sk/modules/test_util';
import { SearchCriteria } from './search-controls-sk/search-controls-sk';

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
    expect(detailHref({ source_type: 'my-corpus', name: 'my-test' }, aDigest)).to.equal(
      '/detail?grouping=name%3Dmy-test%26source_type%3Dmy-corpus'
        + '&digest=aaab78c9711cb79197d47f448ba51338',
    );
    expect(detailHref({ source_type: 'my-corpus', name: 'my-test' }, aDigest, '12345', 'gerrit')).to.equal(
      '/detail?grouping=name%3Dmy-test%26source_type%3Dmy-corpus'
        + '&digest=aaab78c9711cb79197d47f448ba51338&changelist_id=12345&crs=gerrit',
    );
  });
});

describe('diffPageHref', () => {
  it('returns a path with the digests in the expected order', () => {
    expect(diffPageHref({ source_type: 'my-corpus', name: 'my-test' }, aDigest, bDigest)).to.equal(
      '/diff?grouping=name%3Dmy-test%26source_type%3Dmy-corpus'
        + '&left=aaab78c9711cb79197d47f448ba51338&right=bbb8b07beb4e1247c2cbafdb92b93e55',
    );
    // order matters
    expect(diffPageHref({ source_type: 'my-corpus', name: 'my-test' }, bDigest, aDigest)).to.equal(
      '/diff?grouping=name%3Dmy-test%26source_type%3Dmy-corpus'
        + '&left=bbb8b07beb4e1247c2cbafdb92b93e55&right=aaab78c9711cb79197d47f448ba51338',
    );
  });
  it('supports an optional changelist id', () => {
    expect(
      diffPageHref(
        { source_type: 'my-corpus', name: 'my-test' },
        aDigest,
        bDigest,
        '123456',
        'github',
      ),
    ).to.equal(
      '/diff?grouping=name%3Dmy-test%26source_type%3Dmy-corpus'
        + '&left=aaab78c9711cb79197d47f448ba51338'
        + '&right=bbb8b07beb4e1247c2cbafdb92b93e55&changelist_id=123456&crs=github',
    );
  });
});

describe('clusterPageHref', () => {
  const grouping = { source_type: 'my-corpus', name: 'my-test' };
  const searchCriteria: SearchCriteria = {
    corpus: 'my-corpus',
    includeDigestsNotAtHead: true,
    includeIgnoredDigests: true,
    includeNegativeDigests: true,
    includePositiveDigests: true,
    includeUntriagedDigests: true,
    leftHandTraceFilter: {
      foo: ['alpha', 'beta'],
      bar: ['gamma', 'epsilon'],
    },
    rightHandTraceFilter: {
      baz: ['omega'],
    },
    minRGBADelta: 0,
    maxRGBADelta: 10,
    mustHaveReferenceImage: true,
    sortOrder: 'ascending',
  };

  it('returns a valid link without clID/crs', () => {
    expect(clusterPageHref(grouping, searchCriteria)).to.equal('/cluster'
      + '?grouping=name%3Dmy-test%26source_type%3Dmy-corpus&corpus=my-corpus'
      + '&include_ignored=true'
      + '&left_filter='
      + '&max_rgba=10'
      + '&min_rgba=0'
      + '&negative=true'
      + '&not_at_head=true'
      + '&positive=true'
      + '&reference_image_required=true'
      + '&right_filter=baz%3Domega'
      + '&sort=ascending'
      + '&untriaged=true');
  });

  it('returns a valid link with clID/crs', () => {
    expect(clusterPageHref(grouping, searchCriteria, 'my-cl', 'my-crs')).to.equal('/cluster'
      + '?grouping=name%3Dmy-test%26source_type%3Dmy-corpus&corpus=my-corpus'
      + '&include_ignored=true'
      + '&left_filter='
      + '&max_rgba=10'
      + '&min_rgba=0'
      + '&negative=true'
      + '&not_at_head=true'
      + '&positive=true'
      + '&reference_image_required=true'
      + '&right_filter=baz%3Domega'
      + '&sort=ascending'
      + '&untriaged=true'
      + '&changeListID=my-cl'
      + '&crs=my-crs');
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
