import './index.js'

let container = document.createElement("div");
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('select-sk', function() {
  describe('defaults', function() {
    it('Test defaults', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        let s = document.createElement('select-sk');
        container.appendChild(s);
        assert.equal(-1, s._selection);
      })
    });
  });

  describe('click', function() {
    it('changes selector', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        container.innerHTML = `
        <select-sk id=select>
          <div id=a></div>
          <div id=b></div>
        </select>
        `;
        let s = container.firstElementChild;
        let a = s.querySelector('#a');
        let b = s.querySelector('#b');
        a.click();
        assert.equal(0, s._selection);
        assert.isTrue(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        b.click();
        assert.equal(1, s._selection);
        assert.isFalse(a.hasAttribute('selected'));
        assert.isTrue(b.hasAttribute('selected'));
      })
    });
  });
});
