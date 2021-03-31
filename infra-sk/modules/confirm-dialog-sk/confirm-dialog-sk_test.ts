import './index'
import { ConfirmDialogSk } from './confirm-dialog-sk';
import { setUpElementUnderTest } from '../test_util';
import { expect } from 'chai';

function invertPromise(p: Promise<any>) {
  return p.then(
    (x) => {throw x},
    (x) => x
  );
}

describe('confirm-dialog-sk', () => {
  const newElement = setUpElementUnderTest<ConfirmDialogSk>('confirm-dialog-sk');

  let confirmDialogSk: ConfirmDialogSk;

  beforeEach(() => {
    confirmDialogSk = newElement();
  })

  describe('promise', () => {
    it('resolves when OK is clicked', () => {
      const promise = confirmDialogSk.open('Testing');
      const button = confirmDialogSk.querySelector<HTMLButtonElement>('button.confirm')!;
      expect(button.textContent).to.equal('OK');
      expect(confirmDialogSk.querySelector('.message')?.textContent).to.equal('Testing');
      button.click()
      return promise; // Return the promise and let Mocha check that it resolves.
    });

    it('rejects when Cancel is clicked', () => {
      const promise = confirmDialogSk.open("Testing");
      const button = confirmDialogSk.querySelector<HTMLButtonElement>('button.dismiss')!;
      expect(button.textContent).to.equal('Cancel');
      button.click();
      return invertPromise(promise);
    });
  });

  describe('appearance', () => {
    it('sets shown on the inner dialog-sk', () => {
      expect(confirmDialogSk.querySelector('dialog')?.hasAttribute('open')).to.be.false;
      confirmDialogSk.open('whatever');
      expect(confirmDialogSk.querySelector('dialog')?.hasAttribute('open')).to.be.true;
    });
  })
});
