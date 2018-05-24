import './index.js'
import './app-sk.scss'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('app-sk', function() {
  describe('creation', function() {

    it('adds a hamburger button to the header', function() {
      return window.customElements.whenDefined('app-sk').then(() => {
        container.innerHTML = `
        <app-sk>
          <header></header>
          <aside></aside>
          <main></main>
          <footer></footer>
        </app-sk>`;
        let ele = container.firstElementChild;
        let header = ele.querySelector('header');
        assert.equal(header.children.length, 1);
        assert.equal(header.firstElementChild.tagName, 'BUTTON');
      })
    });

    it('handles there being no header', function() {
      return window.customElements.whenDefined('app-sk').then(() => {
        container.innerHTML = `<app-sk></app-sk>`;
        let ele = container.firstElementChild;
        assert.equal(ele.children.length, 0,
                    'Nothing should be added when there is no header');
      })
    });

    it('handles there being no sidebar', function() {
      return window.customElements.whenDefined('app-sk').then(() => {
        container.innerHTML = `<app-sk><header></header></app-sk>`;
        let ele = container.firstElementChild;
        assert.equal(ele.children.length, 1);
        let header = ele.querySelector('header');
        assert.equal(header.children.length, 0,
                    'Nothing should be added when there is no sidebar');
      })
    });
  });

  describe('sidebar', function() {

    it('should toggle', function() {
      return window.customElements.whenDefined('app-sk').then(() => {
        container.innerHTML = `
        <app-sk>
          <header></header>
          <aside></aside>
        </app-sk>`;
        let ele = container.firstElementChild;
        let header = ele.querySelector('header');
        let sidebar = ele.querySelector('aside');
        let btn = header.firstElementChild;
        assert.isFalse(sidebar.classList.contains('shown'));
        btn.click();
        assert.isTrue(sidebar.classList.contains('shown'));
        btn.click();
        assert.isFalse(sidebar.classList.contains('shown'));
      })
    });
  });

});
