import './index.js'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = '';
});

describe('multi-select-sk', function() {
  describe('selection property', function() {
    it('has a default value', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `<multi-select-sk></multi-select-sk>`;
        let s = container.firstElementChild;
        assert.deepEqual([], s._selection);
        assert.deepEqual([], s.selection);
      })
    });

    it('changes based on children', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select>
          <div id=a></div>
          <div id=b selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.deepEqual([1], s._selection);
        assert.deepEqual([1], s.selection);
      })
    });

    it('can go back to []', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select>
          <div id=a></div>
          <div id=b selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        s.selection = [];
        assert.deepEqual([], s.selection);
        assert.isFalse(s.querySelector('#b').hasAttribute('selected'));
      })
    });

    it('treats null and undefined as []', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        container.innerHTML = `
        <select-sk id=select>
          <div id=a></div>
          <div id=b selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        s.selection = null;
        assert.equal(-1, s.selection);
        assert.isFalse(s.querySelector('#b').hasAttribute('selected'));

        s.selection = 0;
        assert.equal(0, s.selection);
        assert.isTrue(s.querySelector('#a').hasAttribute('selected'));

        s.selection = undefined;
        assert.equal(-1, s.selection);
      })
    });

   it('changes the selected attributes on the children', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select>
          <div id=a></div>
          <div id=b></div>
        </select>
        `;
        let s = container.firstElementChild;
        let a = s.querySelector('#a');
        let b = s.querySelector('#b');
        s.selection = [0];
        assert.deepEqual([0], s.selection);
        assert.isTrue(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        s.selection = [0, 1];

        assert.deepEqual([0, 1], s.selection);
        assert.isTrue(a.hasAttribute('selected'));
        assert.isTrue(b.hasAttribute('selected'));
      });
    });

    it('is stays fixed when disabled', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select>
          <div id=a></div>
          <div id=b selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.deepEqual([1], s._selection);
        assert.deepEqual([1], s.selection);
        s.disabled = true;
        s.selected = [0];
        assert.deepEqual([1], s._selection);
        assert.deepEqual([1], s.selection);
        assert.isTrue(s.hasAttribute('disabled'));
      })
    });

    it('gets updated when re-enabled', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select disabled>
          <div id=a></div>
          <div id=b selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.deepEqual([], s._selection);
        assert.deepEqual([], s.selection);
        s.disabled = false;
        assert.deepEqual([1], s._selection);
        assert.deepEqual([1], s.selection);
        assert.isFalse(s.hasAttribute('disabled'));
      })
    });

    it('is always sorted when read', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select>
          <div></div>
          <div></div>
          <div></div>
          <div></div>
          <div></div>
          <div></div>
        </select>`;
        let s = container.firstElementChild;
        s.selection = [5,4,0,2];
        assert.deepEqual([0,2,4,5], s.selection);
      });
    });
  }); // end describe('selection property')


  describe('click', function() {
    it('changes selection in an additive fashion', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select>
          <div id=a></div>
          <div id=b></div>
          <div id=c></div>
        </select>
        `;
        let s = container.firstElementChild;
        let a = s.querySelector('#a');
        let b = s.querySelector('#b');
        a.click();
        assert.deepEqual([0], s.selection);
        assert.isTrue(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        assert.isFalse(c.hasAttribute('selected'));
        b.click();
        assert.deepEqual([0, 1], s.selection);
        assert.isTrue(a.hasAttribute('selected'));
        assert.isTrue(b.hasAttribute('selected'));
        assert.isFalse(c.hasAttribute('selected'));
        // unselect
        b.click();
        assert.deepEqual([0], s.selection);
        assert.isTrue(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        assert.isFalse(c.hasAttribute('selected'));
      })
    });

    it('ignores clicks when disabled', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select disabled>
          <div id=a></div>
          <div id=b></div>
          <div id=c></div>
        </select>
        `;
        let s = container.firstElementChild;
        let a = s.querySelector('#a');
        let b = s.querySelector('#b');
        a.click();
        assert.deepEqual([], s.selection);
        assert.isFalse(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        assert.isFalse(c.hasAttribute('selected'));
        b.click();
        assert.deepEqual([], s.selection);
        assert.isFalse(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        assert.isFalse(c.hasAttribute('selected'));
        // unselect
        b.click();
        assert.deepEqual([], s.selection);
        assert.isFalse(a.hasAttribute('selected'));
        assert.isFalse(b.hasAttribute('selected'));
        assert.isFalse(c.hasAttribute('selected'));
      })
    });
  }); // end describe('click')

  describe('addition of children', function() {
    it('updates selection when a selected child is added', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select>
          <div></div>
          <div></div>
          <div></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.deepEqual([], s.selection);
        let div = document.createElement('div');
        div.setAttribute('selected', '');
        s.appendChild(div)
        div = document.createElement('div');
        s.appendChild(div)
        div = document.createElement('div');
        div.setAttribute('selected', '');
        s.appendChild(div)
        // Need to do the check post microtask so the mutation observer gets a
        // chance to fire.
        return Promise.resolve().then(() => {
          assert.deepEqual([3, 5], s.selection);
        });
      });
    });

    it('does not check children when disabled', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select disabled>
          <div></div>
          <div></div>
          <div></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.deepEqual([], s.selection);
        let div = document.createElement('div');
        div.setAttribute('selected', '');
        s.appendChild(div)
        div = document.createElement('div');
        s.appendChild(div)
        div = document.createElement('div');
        div.setAttribute('selected', '');
        s.appendChild(div)
        // Need to do the check post microtask so the mutation observer gets a
        // chance to fire.
        return Promise.resolve().then(() => {
          assert.deepEqual([], s.selection);
        });
      });
    });
  });  // end describe('addition of children')

  describe('mutation of child selected attribute', function() {
    it('does not update selection', function() {
      return window.customElements.whenDefined('multi-select-sk').then(() => {
        container.innerHTML = `
        <multi-select-sk id=select>
          <div></div>
          <div></div>
          <div id=d2 selected></div>
        </select>
        `;
        let s = container.firstElementChild;
        assert.deepEqual([2], s.selection);
        s.querySelector('#d2').removeAttribute('selected');
        // Need to do the check post microtask so the mutation observer gets a
        // chance to fire.
        return Promise.resolve().then(() => {
          assert.deepEqual([2], s.selection);
        });
      })
    });
  }); // end describe('mutation of child selected attribute

});
