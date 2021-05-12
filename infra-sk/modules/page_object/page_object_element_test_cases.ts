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
}

/**
 * Sets up the PageObjectElement test cases shared by its in-browser and Puppeteer test suites.
 *
 * Any test cases for behaviors that apply to both the in-browser and Puppeteer environments should
 * be place here. Test cases for environment-specific behaviors, if any, should be placed in
 * page_object_element_karma_test.ts or page_object_element_nodejs_test.ts.
 */
export const describePageObjectElement = (testBed: TestBed) => {
  it('supports isEmpty', async () => {
    const poe = await testBed.setUpPageObjectElement('<div>Hello, world!</div>');
    expect(await poe.isEmpty()).to.be.false;
    const nullPromise = new Promise<null>((resolve) => resolve(null));
    expect(await (new PageObjectElement(nullPromise)).isEmpty()).to.be.true;
  });

  it('supports innerText', async () => {
    const poe = await testBed.setUpPageObjectElement('<div>Hello, world!</div>');
    expect(await poe.innerText).to.equal('Hello, world!');
  });

  it('supports isInnerTextEqualTo', async () => {
    const poe = await testBed.setUpPageObjectElement('<div>Hello, world!</div>');
    expect(await poe.isInnerTextEqualTo('Hello, world!')).to.be.true;
    expect(await poe.isInnerTextEqualTo(' Hello, world!')).to.be.false;
    expect(await poe.isInnerTextEqualTo('Hello, world! ')).to.be.false;
    expect(await poe.isInnerTextEqualTo('Goodbye')).to.be.false;
  });

  it('supports className', async () => {
    const poe = await testBed.setUpPageObjectElement('<div class="hello world"></div>');
    expect(await poe.className).to.equal('hello world');
  });

  it('supports hasClassName', async () => {
    const poe = await testBed.setUpPageObjectElement('<div class="hello world"></div>');
    expect(await poe.hasClassName('hello')).to.be.true;
    expect(await poe.hasClassName('world')).to.be.true;
    expect(await poe.hasClassName('goodbye')).to.be.false;
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

  it('supports getAttribute', async () => {
    let poe = await testBed.setUpPageObjectElement(`<a>Click me!</a>`);
    expect(await poe.getAttribute('href')).to.be.null;

    poe = await testBed.setUpPageObjectElement(`<a href="/hello-world">Click me!</a>`);
    expect(await poe.getAttribute('href')).to.equal('/hello-world');
  });

  it('supports value', async () => {
    const poe = await testBed.setUpPageObjectElement('<input type="text" value="hello"/>');
    expect(await poe.value).to.equal('hello');
  });

  it('supports enterValue', async () => {
    const poe = await testBed.setUpPageObjectElement(`
      <input type="text"
             oninput="this.setAttribute('input-event-dispatched', true)"
             onchange="this.setAttribute('change-event-dispatched', true)"/>`);

    const wasInputEventDispatched =
      () => testBed.evaluate((el: HTMLElement) => el.hasAttribute('input-event-dispatched'));
    const wasChangeEventDispatched =
      () => testBed.evaluate((el: HTMLElement) => el.hasAttribute('change-event-dispatched'));

    expect(await wasInputEventDispatched()).to.be.false;
    expect(await wasChangeEventDispatched()).to.be.false;
    await poe.enterValue('hello');
    expect(await testBed.evaluate((el) => (el as HTMLInputElement).value)).to.equal('hello');
    expect(await wasInputEventDispatched()).to.be.true;
    expect(await wasChangeEventDispatched()).to.be.true;
  });

  it('supports typeKey', async () => {
    const poe = await testBed.setUpPageObjectElement(`
      <input type="text"
             onkeydown="this.setAttribute('keydown-event-key', event.key)"
             onkeypress="this.setAttribute('keypress-event-key', event.key)"
             onkeyup="this.setAttribute('keyup-event-key', event.key)"/>`);

    const keydownEventKey =
      () => testBed.evaluate((el: HTMLElement) => el.getAttribute('keydown-event-key'));
    const keypressEventKey =
      () => testBed.evaluate((el: HTMLElement) => el.getAttribute('keypress-event-key'));
    const keyupEventKey =
      () => testBed.evaluate((el: HTMLElement) => el.getAttribute('keyup-event-key'));

    expect(await keydownEventKey()).to.be.null;
    expect(await keypressEventKey()).to.be.null;
    expect(await keyupEventKey()).to.be.null;
    await poe.typeKey('a');
    expect(await keydownEventKey()).to.equal('a');
    expect(await keypressEventKey()).to.equal('a');
    expect(await keyupEventKey()).to.equal('a');
  });

  it('supports applyFnToDOMNode', async () => {
    const poe = await testBed.setUpPageObjectElement('<div>Hello, world!</div>');
    const result =
      await poe.applyFnToDOMNode(
        (el, prefix, suffix) => `${prefix}${el.innerText}${suffix}`,
        'The contents are: "',
        '". That is all.');
    expect(result).to.equal('The contents are: "Hello, world!". That is all.');
  });

  describe('selector functions', () => {
    let poe: PageObjectElement;

    beforeEach(async () => {
      poe = await testBed.setUpPageObjectElement(`
        <div>
          <span class=salutation>Hello</span>,
          <span class=name>World</span>!
        </div>
      `);
    });

    it('supports bySelector', async () => {
      expect(await poe.bySelector('p').isEmpty()).to.be.true;
      expect(await poe.bySelector('span').isEmpty()).to.be.false;
      expect(await poe.bySelector('span.name').innerText).to.equal('World');
    });

    it('supports bySelectorAll', async () => {
      expect(await poe.bySelectorAll('p').length).to.equal(0);
      expect(await poe.bySelectorAll('span').length).to.equal(2);
      expect(await poe.bySelectorAll('span').map((el) => el.innerText))
          .to.deep.equal(['Hello', 'World']);
    });
  });
};
