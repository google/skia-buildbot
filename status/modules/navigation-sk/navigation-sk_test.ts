import './index';
import { NavigationSk } from './navigation-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { SetTestSettings } from '../settings';
import { $, $$ } from 'common-sk/modules/dom';

describe('navigation-sk', () => {
  const newInstance = setUpElementUnderTest<NavigationSk>('navigation-sk');

  let element: NavigationSk;
  beforeEach(() => {
    element = newInstance((el: NavigationSk) => {
      SetTestSettings({
        swarmingUrl: 'example.com/swarming',
        taskSchedulerUrl: 'example.com/ts',
        defaultRepo: 'skia',
        repos: new Map([
          ['skia', 'https://skia.googlesource.com/skia/+show/'],
          ['infra', 'https://skia.googlesource.com/buildbot/+show/'],
          ['skcms', 'https://skia.googlesource.com/skcms/+show/'],
        ]),
      });
    });
  });

  describe('display', () => {
    it('navigation items', () => {
      expect($('.item', element)).to.have.length(5);
      expect($('.item', element).map((e) => (e as HTMLElement).innerText)).to.deep.equal([
        ' Repo: skia ',
        ' Repo: infra ',
        ' Repo: skcms ',
        ' Swarming Bots ',
        ' Capacity Stats',
      ]);
    });
  });

  describe('interaction', async () => {
    it('triggers repo-changed', async () => {
      const ep = eventPromise('repo-changed');
      (<HTMLAnchorElement>$('.item', element)[1]).click();
      const event = (await ep) as any;
      expect(event).to.deep.include({ detail: { repo: 'infra' } });
    });
  });
});
