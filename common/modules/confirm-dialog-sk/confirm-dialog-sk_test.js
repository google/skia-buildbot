import './index.js'

let container = document.createElement("div");
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('confirm-dialog-sk', function() {
  describe('ok', function() {
    it('when click OK', function() {
      return window.customElements.whenDefined('confirm-dialog-sk').then(() => {
        container.innerHTML = `<confirm-dialog-sk></confirm-dialog-sk>`;
        let s = container.firstElementChild;
        let p = s.open("Testing");
        let b = s.querySelectorAll('button')[1];
        assert.equal(b.textContent, 'OK');
        b.click();
        return p;
      })
    });
  });

});
