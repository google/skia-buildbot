import './index.js'

import { $, $$ } from 'common-sk/modules/dom'

describe('pagination-sk', () => {

  // A reusable HTML element in which we create our element under test.
  const container = document.createElement('div');
  document.body.appendChild(container);

  afterEach(function() {
    container.innerHTML = '';
  });

  // calls the test callback with an element under test 'ele'.
  // We can't put the describes inside the whenDefined callback because
  // that doesn't work on Firefox (and possibly other places).
  function createElement(test) {
    return window.customElements.whenDefined('pagination-sk').then(() => {
      container.innerHTML = `
        <pagination-sk offset=0 total=127 page_size=20></pagination-sk>`;
      expect(container.firstElementChild).to.not.be.null;
      test(container.firstElementChild);
    });
  }

  //===============TESTS START====================================

  describe('html layout', () => {
    it('has three buttons', () => {
      return createElement((ele) => {
        const btns = $('button', ele);
        expect(btns.length).to.equal(3);
        // backward
        expect(btns[0].hasAttribute('disabled')).to.be.true;
        // forward
        expect(btns[1].hasAttribute('disabled')).to.be.false;
        // forward+5
        expect(btns[2].hasAttribute('disabled')).to.be.false;

        ele.offset = 20;
        ele._render();
        expect(btns[0].hasAttribute('disabled')).to.be.false;
        expect(btns[1].hasAttribute('disabled')).to.be.false;
        expect(btns[2].hasAttribute('disabled')).to.be.false;

        ele.offset = 40;
        ele._render();
        expect(btns[0].hasAttribute('disabled')).to.be.false;
        expect(btns[1].hasAttribute('disabled')).to.be.false;
        expect(btns[2].hasAttribute('disabled')).to.be.true;

        ele.offset = 120;
        ele._render();
        expect(btns[0].hasAttribute('disabled')).to.be.false;
        expect(btns[1].hasAttribute('disabled')).to.be.true;
        expect(btns[2].hasAttribute('disabled')).to.be.true;
      });
    });

    it('displays the page count', () => {
      return createElement((ele) => {
        const cnt = $$('.counter', ele);
        expect(cnt).to.not.be.null;
        expect(cnt.textContent).to.have.string('page 1');
      });
    });

    it('has several properties', () => {
      return createElement((ele) => {
        expect(ele.total).to.equal(127);
        expect(ele.offset).to.equal(0);
        expect(ele.page_size).to.equal(20);
      });
    });
  });// end describe('html layout')

  describe('paging behavior', () => {
    it('creates page events', (done) => {
      createElement((ele) => {
        ele.offset = 20;
        ele._render();
        const btns = $('button', ele);
        expect(btns.length).to.equal(3);
        const bck =  btns[0];
        const fwd =  btns[1];
        const pls5 = btns[2];

        let d = 0;
        const deltas = [1 ,-1, 5];
        ele.addEventListener('page-changed', (e) => {
          expect(e.detail.delta).to.equal(deltas[d]);
          d++;
          if (d === 3) {
            done();
          }
        });
        fwd.click();
        bck.click();
        pls5.click();
      });
    });
  }); // end describe('paging behavior')

});