import { expect } from 'chai';
import { PageObjectElement } from './page_object_element';
import { Serializable } from 'puppeteer';

/**
 * A test bed for PageObjectElement test cases that must run both in-browser and on Puppeteer.
 *
 * The methods in this interface allow writing PageObjectElement test cases in an
 * environment-independent way. Each environment (i.e. the in-browser and Puppeteer test suites)
 * must provide its own implementation.
 */
export interface TestBed {
  /**
   * Injects the given HTML into a test page, and returns a new PageObjectElement wrapping the
   * top-level element in said HTML.
   *
   * The HTML must have exactly one top-level element.
   *
   * @example
   * "<div>Hello, world!</div>"                // Valid example.
   * "<span>Hello</span> <span>World!</span>"  // Invalid example.
   */
  setUpPageObjectElement(html: string): Promise<PageObjectElement>;

  /**
   * Evaluates the given function inside the test page, passing as the first argument the
   * HTMLElement wrapped by the PageObjectElement returned by the most recent call to
   * setUpPageObjectElement().
   */
  evaluate<T extends Serializable | void = void>(fn: (el: HTMLElement) => T): Promise<T>;
};

/**
 * Sets up the PageObjectElement test cases shared by its in-browser and Puppeteer test suites.
 *
 * Any test cases for behaviors that apply to both the in-browser and Puppeteer environments should
 * be place here. Test cases for environment-specific behaviors, if any, should be placed in
 * page_object_element_test.ts or page_object_element_puppeteer_test.ts.
 */
export const describePageObjectElement = (testBed: TestBed) => {
  it('supports innerText', async () => {
    const poe = await testBed.setUpPageObjectElement('<div>Hello, world!</div>');
    expect(await poe.innerText).to.equal('Hello, world!');
  });

  it('supports className', async () => {
    const poe = await testBed.setUpPageObjectElement('<div class="hello world"></div>');
    expect(await poe.className).to.equal('hello world');
  });

  it('supports focus', async () => {
    const poe = await testBed.setUpPageObjectElement(`<button>Hello, world!</button>`);
    const isFocused = () => testBed.evaluate((el) => document.activeElement === el);

    expect(await isFocused()).to.be.false;
    await poe.focus();
    expect(await isFocused()).to.be.true;
  });

  it('supports click', async () => {
    const poe =
      await testBed.setUpPageObjectElement(
        `<button onclick="this.setAttribute('clicked', true)">Click me!</button>`);
    const wasClicked = () => testBed.evaluate((el: HTMLElement) => el.hasAttribute('clicked'));

    expect(await wasClicked()).to.be.false;
    await poe.click();
    expect(await wasClicked()).to.be.true;
  });

  it('supports hasAttribute', async () => {
    let poe = await testBed.setUpPageObjectElement(`<div></div>`);
    expect(await poe.hasAttribute('hello')).to.be.false;

    poe = await testBed.setUpPageObjectElement(`<div hello></div>`);
    expect(await poe.hasAttribute('hello')).to.be.true;
  });

  it('supports value', async () => {
    const poe = await testBed.setUpPageObjectElement('<input type="text" value="hello"/>');
    expect(await poe.value).to.equal('hello');
  });

  it('supports setValue', async () => {
    const poe = await testBed.setUpPageObjectElement('<input type="text"/>');
    await poe.setValue('hello');
    expect(await testBed.evaluate((el) => (el as HTMLInputElement).value)).to.equal('hello');
  });

  it('supports evalDom', async () => {
    const poe = await testBed.setUpPageObjectElement('<div>Hello, world!</div>');
    const result =
      await poe.evalDom(
        (el, prefix, suffix) => `${prefix}${el.innerText}${suffix}`,
        'The contents are: "',
        '". That is all.');
    expect(result).to.equal('The contents are: "Hello, world!". That is all.');
  });

  describe('query selector functions', () => {
    let poe: PageObjectElement;

    beforeEach(async () => {
      poe = await testBed.setUpPageObjectElement(`
        <div>
          <span class=salutation>Hello</span>,
          <span class=name>World</span>!
        </div>
      `);
    });

    it('supports querySelector and $$', async () => {
      // querySelector.
      expect(await poe.querySelector('p')).to.be.null;
      expect(await poe.querySelector('span')).to.not.be.null;
      expect(await (await poe.querySelector('span.name'))!.innerText).to.equal('World');

      // $$.
      expect(await poe.$$('p')).to.be.null;
      expect(await poe.$$('span')).to.not.be.null;
      expect(await (await poe.$$('span.salutation'))!.innerText).to.equal('Hello');
    });

    it('supports querySelectorAll and $', async () => {
      const innerTexts =
        async (els: Promise<PageObjectElement[]>) =>
          await Promise.all((await els).map((el) => el.innerText));

      // querySelectorAll.
      expect(await poe.querySelectorAll('p')).to.have.length(0);
      expect(await poe.querySelectorAll('span')).to.have.length(2);
      expect(await innerTexts(poe.querySelectorAll('span'))).to.deep.equal(['Hello', 'World']);

      // $.
      expect(await poe.$('p')).to.have.length(0);
      expect(await poe.$('span')).to.have.length(2);
      expect(await innerTexts(poe.$('span'))).to.deep.equal(['Hello', 'World']);
    });

    it('supports $$apply', async () => {
      expect(await poe.$$apply('span.name', (el: PageObjectElement) => el.innerText))
        .to.equal('World');
    });

    it('supports $$evalDom', async () => {
      expect(await poe.$$evalDom('span.name', (el: HTMLElement) => el.innerText))
        .to.equal('World');
    });

    it('supports $map', async () => {
      expect(await poe.$map('span', (el) => el.innerText)).to.deep.equal(['Hello', 'World']);
    });

    it('supports $each', async () => {
      const text: string[] = [];
      await poe.$each('span', async (el) => {
        text.push(await el.innerText);
      });
      expect(text).to.deep.equal(['Hello', 'World']);
    });

    it('supports $find', async () => {
      expect(await poe.$find('span', async (el) => (await el.className) === 'unknown')).to.be.null;

      const span = await poe.$find('span', async (el) => (await el.className) === 'name');
      expect(await span!.innerText).to.equal('World');
    });
  });
};
