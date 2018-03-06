import './index.js'

let container = document.createElement("div");
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('select-sk', function() {
  describe('defaults', function() {
    it('has default values', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        container.innerHTML = `<select-sk></select-sk>`;
        let s = container.firstElementChild;
        assert.equal(-1, s._selection);
        assert.equal(-1, s.selection);
      })
    });
  });

  describe('defaults', function() {
    it('changes default values based on children', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        container.innerHTML = `
        <select-sk id=select>
          <div id=a></div>
          <div id=b selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.equal(1, s._selection);
        assert.equal(1, s.selection);
      })
    });
  });

  describe('click', function() {
    it('changes selection', function() {
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
        assert.equal(0, s.selection);
        assert.isTrue(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        b.click();
        assert.equal(1, s.selection);
        assert.isFalse(a.hasAttribute('selected'));
        assert.isTrue(b.hasAttribute('selected'));
      })
    });
  });

  describe('selection property', function() {
    it('changes selection', function() {
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
        s.selection = 0;
        assert.equal(0, s.selection);
        assert.isTrue(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        s.selection = 1;
        assert.equal(1, s.selection);
        assert.isFalse(a.hasAttribute('selected'));
        assert.isTrue(b.hasAttribute('selected'));
      })
    });
  });

  describe('click', function() {
    it('updates selection based on mutations', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        container.innerHTML = `
        <select-sk id=select>
          <div></div>
          <div></div>
          <div></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.equal(-1, s.selection);
        let div = document.createElement('div');
        div.setAttribute('selected', '');
        s.appendChild(div)
        // Need to do the check post microtask so the mutation observer gets a
        // chance to fire.
        return Promise.resolve().then(() => {
          assert.equal(3, s.selection);
        });
      })
    });
  });

});
