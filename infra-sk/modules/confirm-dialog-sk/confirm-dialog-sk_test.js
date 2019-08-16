import './index.js'
import { $$ } from 'common-sk/modules/dom'

let container = document.createElement('div');
document.body.appendChild(container);

afterEach(function() {
  container.innerHTML = "";
});

function invertPromise(p) {
  return p.then(
    (x) => {throw x},
    (x) => x
  );
};

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
        // Return the promise and let Mocha check that it resolves.
        return promise;
      });
    });

    it('rejects when Cancel is clicked', function() {
      return window.customElements.whenDefined('confirm-dialog-sk').then(() => {
        container.innerHTML = `<confirm-dialog-sk></confirm-dialog-sk>`;
        let dialog = container.firstElementChild;
        let promise = dialog.open("Testing");
        let button = dialog.querySelectorAll('button')[0];
        assert.equal(button.textContent, 'Cancel');
        button.click();
        return invertPromise(promise);
      });
    });
  });

  describe('appearance', function() {
    it('sets shown on the inner dialog-sk', function() {
      return window.customElements.whenDefined('confirm-dialog-sk').then(() => {
        container.innerHTML = `<confirm-dialog-sk></confirm-dialog-sk>`;
        let dialog = container.firstElementChild;

        assert.equal($$('dialog', dialog).hasAttribute('open'), false);
        dialog.open('whatever');
        assert.equal($$('dialog', dialog).hasAttribute('open'), true);
      });
    });
  })

});
