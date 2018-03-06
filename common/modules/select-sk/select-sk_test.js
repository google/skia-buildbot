import './index.js'

let container = document.createElement("div");
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('Test select-sk custom element.',
  function() {
    it('Test defaults', function() {
      return window.customElements.whenDefined('select-sk').then(() => {
        let s = document.createElement('select-sk');
        container.appendChild(s);
        assert.equal(-1, s._selection);
        container.removeChild(s);
      })
    });
  }
);
