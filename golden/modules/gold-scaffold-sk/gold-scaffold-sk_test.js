import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { sendBeginTask, sendEndTask } from '../common';

describe('gold-scaffold-sk', () => {
  const newInstance = setUpElementUnderTest('gold-scaffold-sk');

  let goldScaffoldSk;
  beforeEach(() => {
    goldScaffoldSk = newInstance((el) => {
      el.setAttribute('testing_offline', '');
      el.innerHTML = '<div>content</div>';
    });
  });

  describe('html layout', () => {
    it('adds a login-sk element', () => {
      const log = $$('header login-sk', goldScaffoldSk);
      expect(log).to.not.be.null;
    });

    it('adds a sidebar with links', () => {
      const nav = $$('aside nav', goldScaffoldSk);
      expect(nav).to.not.be.null;
      const links = $('a', nav);
      expect(links.length).not.to.equal(0);
    });

    it('puts the content under <main>', () => {
      const main = $$('main', goldScaffoldSk);
      expect(main).to.not.be.null;
      const content = $$('div', main);
      expect(content).to.not.be.null;
      expect(content.textContent).to.equal('content');
    });
  });// end describe('html layout')

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
      const spinner = $$('header spinner-sk', goldScaffoldSk);
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
