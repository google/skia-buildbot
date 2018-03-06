import './index.js'

let container = document.createElement("div");
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('select-sk', function() {
  describe('selection', function() {
    it('has a default value', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        container.innerHTML = `<select-sk></select-sk>`;
        let s = container.firstElementChild;
        assert.equal(-1, s._selection);
        assert.equal(-1, s.selection);
      })
    });
  });

  describe('selection', function() {
    it('changes based on children', function() {
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

  describe('selection', function() {
    it('can go back to -1', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        container.innerHTML = `
        <select-sk id=select>
          <div id=a></div>
          <div id=b selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        s.selection = -1;
        assert.equal(-1, s.selection);
        assert.isFalse(s.querySelector('#b').hasAttribute('selected'));
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

  describe('mutation', function() {
    it('updates selection', function() {
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

  describe('mutation of child selected attribute', function() {
    it('does not update selection', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        container.innerHTML = `
        <select-sk id=select>
          <div></div>
          <div></div>
          <div id=d2 selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.equal(2, s.selection);
        s.querySelector('#d2').removeAttribute('selected');
        // Need to do the check post microtask so the mutation observer gets a
        // chance to fire.
        return Promise.resolve().then(() => {
          assert.equal(2, s.selection);
        });
      })
    });
  });

});
