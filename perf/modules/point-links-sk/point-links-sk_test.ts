import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { PointLinksSk } from './point-links-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CommitNumber } from '../json';

describe('point-links-sk', () => {
  const newInstance = setUpElementUnderTest<PointLinksSk>('point-links-sk');

  let element: PointLinksSk;
  beforeEach(() => {});

  describe('Load links for a commit.', () => {
    beforeEach(() => {
      element = newInstance();
      fetchMock.reset();
    });

    it('With no eligible links.', () => {
      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);
      const keysForCommitRange: string[] = [];
      const keysForUsefulLinks: string[] = [];
      element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks
      );
      assert.isEmpty(element.displayUrls, 'No display urls expected.');
      assert.isEmpty(element.displayTexts, 'No display texts expected.');
    });

    it('With all eligible links but no range.', async () => {
      const keysForCommitRange = ['key1', 'key2'];
      const keysForUsefulLinks = [''];
      const returnLinks = {
        key1: 'https://commit/link1',
        key2: 'https://commit/link2',
      };
      fetchMock.post('/_/details/?results=false', {
        version: 1,
        links: returnLinks,
      });

      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      const expectedLinks = {
        'key1 Commit': 'https://commit/link1',
        'key2 Commit': 'https://commit/link2',
      };
      await element.load(
        currentCommitId,
        prevCommitId,
        'my trace',
        keysForCommitRange,
        keysForUsefulLinks
      );
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With all eligible links and only ranges.', async () => {
      const keysForCommitRange = ['key1', 'key2'];
      const keysForUsefulLinks = [''];
      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      fetchMock.post('/_/details/?results=false', (_url, request) => {
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
        keysForUsefulLinks
      );
      const expectedLinks = {
        'key1 Range': 'https://repoHost/repo1/+log/preLink..curLink',
        'key2 Range': 'https://repoHost/repo2/+log/preLink..curLink',
      };
      assert.deepEqual(expectedLinks, element.displayUrls);
    });

    it('With all eligible links and mixed links and ranges.', async () => {
      const keysForCommitRange = ['key1', 'key2'];
      const keysForUsefulLinks = ['buildKey', 'traceKey'];
      const currentCommitId = CommitNumber(4);
      const prevCommitId = CommitNumber(3);

      fetchMock.post('/_/details/?results=false', (_url, request) => {
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
        keysForUsefulLinks
      );
      const expectedLinks = {
        'key1 Commit': 'https://repoHost/repo1/+/curLink',
        'key2 Range': 'https://repoHost/repo2/+log/preLink..curLink',
        buildKey: 'https://luci/builder/build1',
        traceKey: 'https://traceViewer/trace',
      };
      assert.deepEqual(expectedLinks, element.displayUrls);
    });
  });
});
