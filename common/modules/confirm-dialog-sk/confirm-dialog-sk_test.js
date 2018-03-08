import './index.js'

let container = document.createElement("div");
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

describe('confirm-dialog-sk', function() {
  describe('promise', function() {
    it('resolves when OK is clicked', function() {
      return window.customElements.whenDefined('confirm-dialog-sk').then(() => {
        container.innerHTML = `<confirm-dialog-sk></confirm-dialog-sk>`;
        let dialog = container.firstElementChild;
        let promise = dialog.open("Testing");
        let button = dialog.querySelectorAll('button')[1];
        assert.equal(button.textContent, 'OK');
        button.click();
        return promise;
      })
    });
  });

});
