import './index';

import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { $, $$ } from '../../../infra-sk/modules/dom';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { sendBeginTask, sendEndTask } from '../common';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import { testOnlySetSettings } from '../settings';
import { GoldScaffoldSk } from './gold-scaffold-sk';

describe('gold-scaffold-sk', () => {
  const newInstance = setUpElementUnderTest<GoldScaffoldSk>('gold-scaffold-sk');

  let goldScaffoldSk: GoldScaffoldSk;

  beforeEach(() => {
    testOnlySetSettings({
      title: 'Skia Public',
      baseRepoURL: 'https://skia.googlesource.com/skia.git',
    });
    fetchMock.get('/json/v2/trstatus', JSON.stringify(exampleStatusData));
    goldScaffoldSk = newInstance((el) => {
      el.setAttribute('testing_offline', '');
      el.innerHTML = '<div>content</div>';
    });
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    // Remove fetch mocking to prevent test cases interfering with each other.
    fetchMock.reset();
  });

  describe('html layout', () => {
    it('adds a alogin-sk element', () => {
      const log = $$('header alogin-sk', goldScaffoldSk);
      expect(log).to.not.be.null;
    });

    it('adds a sidebar with links', () => {
      const nav = $$('aside nav', goldScaffoldSk);
      expect(nav).to.not.be.null;
      const links = $('a', nav!);
      expect(links.length).not.to.equal(0);
    });

    it('puts the content under <main>', () => {
      const main = $$<HTMLElement>('main', goldScaffoldSk);
      expect(main).to.not.be.null;
      const content = $$<HTMLDivElement>('div', main!);
      expect(content).to.not.be.null;
      expect(content!.textContent).to.equal('content');
    });
  }); // end describe('html layout')

  describe('spinner and busy property', () => {
    it('becomes busy while there are tasks to be done', () => {
      expect(goldScaffoldSk.busy).to.equal(false);

      sendBeginTask(goldScaffoldSk);
      sendBeginTask(goldScaffoldSk);
      expect(goldScaffoldSk.busy).to.equal(true);

      sendEndTask(goldScaffoldSk);
      sendEndTask(goldScaffoldSk);
      expect(goldScaffoldSk.busy).to.equal(false);
    });

    it('keeps spinner active while busy', () => {
      const spinner = $$<SpinnerSk>('header spinner-sk', goldScaffoldSk)!;
      expect(spinner.active).to.equal(false);

      sendBeginTask(goldScaffoldSk);
      sendBeginTask(goldScaffoldSk);
      expect(spinner.active).to.equal(true);

      sendEndTask(goldScaffoldSk);
      sendEndTask(goldScaffoldSk);
      expect(spinner.active).to.equal(false);
    });

    it('emits a busy-end task when tasks finished', async () => {
      const busyEnd = eventPromise('busy-end');

      sendBeginTask(goldScaffoldSk);
      await new Promise((resolve) => setTimeout(resolve, 10));
      sendEndTask(goldScaffoldSk);

      await busyEnd;
    });
  }); // end describe('spinner and busy property')
});
