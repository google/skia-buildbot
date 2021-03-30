import './index';
import { AppSk } from './app-sk';
import { setUpElementUnderTest } from '../test_util';
import { expect } from 'chai';

describe('app-sk', () => {
  const newInstance = setUpElementUnderTest<AppSk>('app-sk');

  describe('creation', () => {
    it('adds a hamburger button to the header', () => {
      const appSk = newInstance((el) => el.innerHTML = `
        <header></header>
        <aside></aside>
        <main></main>
        <footer></footer>
      `);
      const header = appSk.querySelector('header')!;
      expect(header.children).to.have.length(1);
      expect(header.firstElementChild?.tagName).to.equal('BUTTON');
    });

    it('handles there being no header', () => {
      const appSk = newInstance();
      expect(appSk.children).to.be.empty;
    });

    it('handles there being no sidebar', () => {
      const appSk = newInstance((el) => el.innerHTML = '<header></header>');
      expect(appSk.children).to.have.length(1);
      const header = appSk.querySelector('header')!;
      expect(header.children).to.be.empty;
    });
  });

  describe('sidebar', () => {
    it('should toggle', () => {
      const appSk = newInstance((el) => el.innerHTML = `
        <header></header>
        <aside></aside>
      `);
      const header = appSk.querySelector('header')!;
      const sidebar = appSk.querySelector('aside')!;
      const btn = header?.firstElementChild as HTMLButtonElement;
      expect(sidebar.classList.contains('shown')).to.be.false;
      btn.click();
      expect(sidebar.classList.contains('shown')).to.be.true;
      btn.click();
      expect(sidebar.classList.contains('shown')).to.be.false;
    });
  });
});
