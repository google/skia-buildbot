import './index.js'

import { $, $$ } from 'common-sk/modules/dom'

describe('gold-scaffold-sk', () => {

  // A reusable HTML element in which we create our element under test.
  const container = document.createElement('div');
  document.body.appendChild(container);

  afterEach(function() {
    container.innerHTML = '';
  });

  // calls the test callback with one element 'ele', a created <swarming-app>.
  // We can't put the describes inside the whenDefined callback because
  // that doesn't work on Firefox (and possibly other places).
  function createElement(test) {
    return window.customElements.whenDefined('gold-scaffold-sk').then(() => {
      container.innerHTML = `
          <gold-scaffold-sk testing_offline>
            <div>content</div>
          </gold-scaffold-sk>`;
      expect(container.firstElementChild).to.not.be.null;
      test(container.firstElementChild);
    });
  }

  //===============TESTS START====================================

  describe('html layout', () => {
    it('adds a login-sk element', () => {
      return createElement((ele) => {
        const log = $$('header login-sk', ele);
        expect(log).to.not.be.null;
      });
    });

    it('adds a sidebar with links', () => {
      return createElement((ele) => {
        const nav = $$('aside nav', ele);
        expect(nav).to.not.be.null;
        const links = $('a', nav);
        expect(links.length).not.to.equal(0);
      });
    });

    it('puts the content under <main>', () => {
      return createElement((ele) => {
        const main = $$('main', ele);
        expect(main).to.not.be.null;
        const content = $$('div', main);
        expect(content).to.not.be.null;
        expect(content.textContent).to.equal('content');
      });
    });
  });// end describe('html layout')

  describe('spinner and busy property', () => {
    it('becomes busy while there are tasks to be done', () => {
      return createElement((ele) => {
        expect(ele.busy).to.equal(false);
        ele.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
        ele.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
        expect(ele.busy).to.equal(true);
        ele.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
        expect(ele.busy).to.equal(true);
        ele.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
        expect(ele.busy).to.equal(false);
      });
    });

    it('keeps spinner active while busy', () => {
      return createElement((ele) => {
        const spinner = $$('header spinner-sk', ele);
        expect(spinner.active).to.equal(false);
        ele.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
        ele.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
        expect(spinner.active).to.equal(true);
        ele.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
        expect(spinner.active).to.equal(true);
        ele.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
        expect(spinner.active).to.equal(false);
      });
    });

    it('emits a busy-end task when tasks finished', function(done) {
      createElement((ele) => {
        ele.addEventListener('busy-end', (e) => {
          e.stopPropagation();
          expect(ele.busy).to.equal(false);
          done();
        });
        ele.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));

        setTimeout(()=>{
          ele.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
        }, 10);
      });
    });
  }); // end describe('spinner and busy property')

});