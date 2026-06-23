import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { CommitLinks, PointLinksSk, sanitizeUrl } from './point-links-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CommitNumber } from '../json';

describe('point-links-sk', () => {
  const newInstance = setUpElementUnderTest<PointLinksSk>('point-links-sk');

  let element: PointLinksSk;
  beforeEach(() => {
    element = newInstance();
    fetchMock.reset();
  });

  describe('Load links for a commit.', () => {
    const commitLinks: CommitLinks = {
      cid: CommitNumber(4),
      traceid: 'my trace',
      displayUrls: {
        key4: 'https://commit/link1',
      },
      displayTexts: {
        key4: 'Commit Link',
      },
      fetched: true,
    };
    it('With no eligible links.', async () => {
      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);
      const keysForCommitRange: string[] = [];
      const keysForUsefulLinks: string[] = [];
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      assert.isEmpty(element.displayUrls, 'No display urls expected.');
      assert.isEmpty(element.displayTexts, 'No display texts expected.');
    });

    it('With links already stored.', async () => {
      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);
      const keysForCommitRange: string[] = [];
      const keysForUsefulLinks: string[] = [];
      const expectedLinks = {
        key4: 'https://commit/link1',
      };

      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        [commitLinks]
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With links already stored, but no matching commit number.', async () => {
      const keysForCommitRange = ['key1', 'key2'];
      const keysForUsefulLinks = [''];
      const returnLinks = {
        key1: 'https://commit/link1',
        key2: 'https://commit/link2',
      };
      // Mock /_/details/ as it is now the primary
      fetchMock.post('/_/details/?results=false', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(3);
      const prevCommitId = CommitNumber(2);

      const expectedLinks = {
        key1: 'https://commit/link1',
        key2: 'https://commit/link2',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        [commitLinks]
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With all eligible links but single commit.', async () => {
      const keysForCommitRange = ['key1', 'key2'];
      const keysForUsefulLinks = [''];
      const returnLinks = {
        key1: 'https://commit/link1',
        key2: 'https://commit/link2',
      };
      fetchMock.post('/_/links/', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      const expectedLinks = {
        key1: 'https://commit/link1',
        key2: 'https://commit/link2',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With all eligible links and only ranges.', async () => {
      const keysForCommitRange = ['key1', 'key2'];
      const keysForUsefulLinks = [''];
      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      fetchMock.post('/_/links/', (_url, request) => {
        const requestObj = JSON.parse(request.body!.toString());
        switch (requestObj.cid) {
          case currentCommitId:
            return {
              version: 1,
              links: {
                key1: 'https://repoHost/repo1/+/curLink',
                key2: 'https://repoHost/repo2/+/curLink',
              },
            };
          case prevCommitId:
            return {
              version: 1,
              links: {
                key1: 'https://repoHost/repo1/+/preLink',
                key2: 'https://repoHost/repo2/+/preLink',
              },
            };
          default:
            return {};
        }
      });

      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      const expectedLinks = {
        key1: 'https://repoHost/repo1/+log/preLink..curLink?n=1000',
        key2: 'https://repoHost/repo2/+log/preLink..curLink?n=1000',
      };
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With only useful links.', async () => {
      const keysForCommitRange = [''];
      const keysForUsefulLinks = ['buildKey', 'traceKey'];
      const returnLinks = {
        buildKey: 'https://luci/builder/build1',
        traceKey: 'https://traceViewer/trace',
      };

      fetchMock.post('/_/links/', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      const expectedLinks = {
        buildKey: 'https://luci/builder/build1',
        traceKey: 'https://traceViewer/trace',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('When return link in Fuchsia isntance.', async () => {
      const keysForCommitRange = [''];
      const keysForUsefulLinks = ['Build Log'];
      const returnLinks = {
        'Test stdio': '[Build Log](https://commit/link1)',
      };
      fetchMock.post('/_/links/', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      const expectedLinks = {
        'Build Log': 'https://commit/link1',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With all eligible links and mixed links and ranges.', async () => {
      const keysForCommitRange = ['key1', 'key2'];
      const keysForUsefulLinks = ['buildKey', 'traceKey'];
      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      fetchMock.post('/_/links/', (_url, request) => {
        const requestObj = JSON.parse(request.body!.toString());
        switch (requestObj.cid) {
          case currentCommitId:
            return {
              version: 1,
              links: {
                key1: 'https://repoHost/repo1/+/curLink',
                key2: 'https://repoHost/repo2/+/curLink',
                buildKey: 'https://luci/builder/build1',
                traceKey: 'https://traceViewer/trace',
                extraKey: 'https://randomSite',
              },
            };
          case prevCommitId:
            return {
              version: 1,
              links: {
                key1: 'https://repoHost/repo1/+/curLink',
                key2: 'https://repoHost/repo2/+/preLink',
              },
            };
          default:
            return {};
        }
      });

      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      const expectedLinks = {
        key1: 'https://repoHost/repo1/+/curLink',
        key2: 'https://repoHost/repo2/+log/preLink..curLink?n=1000',
        buildKey: 'https://luci/builder/build1',
        traceKey: 'https://traceViewer/trace',
      };
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With useful links having Git suffix.', async () => {
      const keysForCommitRange = [''];
      const keysForUsefulLinks = ['My Key'];
      const returnLinks = {
        'My Key Git': 'https://link',
      };
      fetchMock.post('/_/links/', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      const expectedLinks = {
        'My Key Git': 'https://link',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('Fallback to /_/links/ if /_/details/ returns null.', async () => {
      const keysForCommitRange = [''];
      const keysForUsefulLinks = ['Detail Key'];
      const returnLinks = {
        'Detail Key': 'https://detail-link',
      };

      // Mock /_/details/ to return null/empty
      fetchMock.post('/_/details/?results=false', {
        version: 1,
        links: null,
      });

      // Mock /_/links/ to return valid links
      fetchMock.post('/_/links/', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      const expectedLinks = {
        'Detail Key': 'https://detail-link',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('Robust key matching with whitespace.', async () => {
      const keysForCommitRange = [''];
      const keysForUsefulLinks = ['My Key'];
      const returnLinks = {
        ' My Key ': 'https://link',
      };
      // Mock /_/details/ as it is now the primary
      fetchMock.post('/_/details/?results=false', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      const expectedLinks = {
        ' My Key ': 'https://link',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With useful links having lowercase git suffix.', async () => {
      const keysForCommitRange = [''];
      const keysForUsefulLinks = ['My Key'];
      const returnLinks = {
        'My Key git': 'https://link',
      };
      fetchMock.post('/_/links/', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      const expectedLinks = {
        'My Key git': 'https://link',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('renders markdown links correctly', async () => {
      const keysForCommitRange = [''];
      const keysForUsefulLinks = ['Benchmark Config'];
      const returnLinks = {
        'Benchmark Config':
          '[Benchmark config](https://chrome-internal.googlesource.com/v8/v8-perf/+/f885626a1c24339760cf4fe96c3d5f4b1ef93048/benchmarks/JetStream/JetStream2.json)',
      };
      fetchMock.post('/_/links/', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks,
        []
      );

      // Wait for LitElement rendering and 'until' directive to resolve.
      await element.updateComplete;
      await new Promise((resolve) => setTimeout(resolve, 0));

      const keyEl = element.querySelector('#tooltip-key');
      const textEl = element.querySelector('#tooltip-text');
      assert.isNotNull(keyEl);
      assert.isNotNull(textEl);
      assert.equal(keyEl!.textContent?.trim(), 'Benchmark Config');

      const linkEl = textEl!.querySelector('a');
      assert.isNotNull(linkEl);
      assert.equal(
        linkEl!.getAttribute('href'),
        'https://chrome-internal.googlesource.com/v8/v8-perf/+/f885626a1c24339760cf4fe96c3d5f4b1ef93048/benchmarks/JetStream/JetStream2.json'
      );
      assert.equal(linkEl!.textContent?.trim(), 'Link');
    });
  });

  describe('sanitizeUrl', () => {
    it('allows safe http/https URLs', () => {
      assert.equal(sanitizeUrl('http://example.com'), 'http://example.com');
      assert.equal(
        sanitizeUrl('https://example.com/foo?bar=baz'),
        'https://example.com/foo?bar=baz'
      );
    });

    it('allows relative URLs', () => {
      assert.equal(sanitizeUrl('/foo/bar'), '/foo/bar');
      assert.equal(sanitizeUrl('baz'), 'baz');
    });

    it('blocks unsafe protocols', () => {
      assert.equal(sanitizeUrl('javascript:alert(1)'), '#');
      assert.equal(sanitizeUrl('data:text/html,<html>'), '#');
      assert.equal(sanitizeUrl('vbscript:msgbox'), '#');
    });
  });
});
