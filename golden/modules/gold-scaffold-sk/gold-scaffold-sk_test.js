import './index.js';

import { $, $$ } from 'common-sk/modules/dom';
import { eventPromise, setUpElementUnderTest } from '../test_util';

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
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('begin-task', { bubbles: true }),
      );
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('begin-task', { bubbles: true }),
      );
      expect(goldScaffoldSk.busy).to.equal(true);
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('end-task', { bubbles: true }),
      );
      expect(goldScaffoldSk.busy).to.equal(true);
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('end-task', { bubbles: true }),
      );
      expect(goldScaffoldSk.busy).to.equal(false);
    });

    it('keeps spinner active while busy', () => {
      const spinner = $$('header spinner-sk', goldScaffoldSk);
      expect(spinner.active).to.equal(false);
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('begin-task', { bubbles: true }),
      );
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('begin-task', { bubbles: true }),
      );
      expect(spinner.active).to.equal(true);
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('end-task', { bubbles: true }),
      );
      expect(spinner.active).to.equal(true);
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('end-task', { bubbles: true }),
      );
      expect(spinner.active).to.equal(false);
    });

    it('emits a busy-end task when tasks finished', async () => {
      const busyEnd = eventPromise('busy-end');
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('begin-task', { bubbles: true }),
      );
      await new Promise((resolve) => setTimeout(resolve, 10));
      goldScaffoldSk.dispatchEvent(
        new CustomEvent('end-task', { bubbles: true }),
      );
      await busyEnd;
    });
  }); // end describe('spinner and busy property')
});
