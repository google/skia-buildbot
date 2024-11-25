import './index';
import fetchMock from 'fetch-mock';

import { expect } from 'chai';
import { $, $$ } from '../../../infra-sk/modules/dom';
// TODO(lovisolo,kjlubick): Add the below to infra-sk.
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CtScaffoldSk } from './ct-scaffold-sk';

describe('ct-scaffold-sk', () => {
  const newInstance = setUpElementUnderTest<CtScaffoldSk>('ct-scaffold-sk');

  let scaffoldSk: CtScaffoldSk;
  beforeEach(() => {
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
    fetchMock.post('begin:/_/get_', 200, { repeat: 5 });
    scaffoldSk = newInstance((el) => {
      el.setAttribute('testing_offline', '');
      el.innerHTML = '<div>content</div>';
    });
  });

  describe('html layout', () => {
    it('adds an alogin-sk element', () => {
      const log = $$('header alogin-sk', scaffoldSk);
      expect(log).to.not.be.null;
    });

    it('adds a sidebar with links', () => {
      const nav = $$('aside nav', scaffoldSk) as HTMLElement;
      expect(nav).to.not.be.null;
      const links = $('a', nav);
      expect(links.length).not.to.equal(0);
    });

    it('puts the content under <main>', () => {
      const main = $$('main', scaffoldSk) as HTMLElement;
      expect(main).to.not.be.null;
      const content = $$('div', main);
      expect(content).to.not.be.null;
      expect(content!.textContent).to.equal('content');
    });
  }); // end describe('html layout')

  describe('spinner and busy property', () => {
    it('becomes busy while there are tasks to be done', () => {
      expect(scaffoldSk.busy).to.equal(false);
      scaffoldSk.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
      scaffoldSk.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
      expect(scaffoldSk.busy).to.equal(true);
      scaffoldSk.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      expect(scaffoldSk.busy).to.equal(true);
      scaffoldSk.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      expect(scaffoldSk.busy).to.equal(false);
    });

    it('keeps spinner active while busy', () => {
      const spinner = $$('header spinner-sk', scaffoldSk) as SpinnerSk;
      expect(spinner.active).to.equal(false);
      scaffoldSk.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
      scaffoldSk.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
      expect(spinner.active).to.equal(true);
      scaffoldSk.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      expect(spinner.active).to.equal(true);
      scaffoldSk.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      expect(spinner.active).to.equal(false);
    });

    it('emits a busy-end task when tasks finished', async () => {
      const busyEnd = eventPromise('busy-end');
      scaffoldSk.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
      await new Promise((resolve) => setTimeout(resolve, 10));
      scaffoldSk.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      await busyEnd;
    });
  }); // end describe('spinner and busy property')
});
