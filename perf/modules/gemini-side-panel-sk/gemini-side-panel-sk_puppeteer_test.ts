import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';

describe('gemini-side-panel-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1280, height: 1024 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('gemini-side-panel-sk')).to.have.length(1);
  });

  it('starts closed', async () => {
    const panel = await testBed.page.$('gemini-side-panel-sk');
    expect(panel).to.not.be.null;
    const right = await testBed.page.evaluate((el) => getComputedStyle(el).right, panel!);
    expect(right).to.equal('-400px');
  });

  it('toggles when toggle() is called', async () => {
    await testBed.page.evaluate(() =>
      (document.querySelector('gemini-side-panel-sk') as any).toggle()
    );
    // Wait for transition
    await new Promise((r) => setTimeout(r, 1000));
    const panel = await testBed.page.$('gemini-side-panel-sk');
    expect(panel).to.not.be.null;
    const right = await testBed.page.evaluate((el) => getComputedStyle(el).right, panel!);
    expect(right).to.equal('0px');
  });

  it('input and send', async () => {
    // Force open to ensure elements are visible
    await testBed.page.evaluate(() => {
      (document.querySelector('gemini-side-panel-sk') as any).open = true;
    });
    await new Promise((r) => setTimeout(r, 1000));

    const panelHandle = await testBed.page.$('gemini-side-panel-sk');
    expect(panelHandle).to.not.be.null;
    const inputHandle = await panelHandle!.evaluateHandle((el) =>
      el.shadowRoot!.querySelector('input')
    );
    const buttonHandle = await panelHandle!.evaluateHandle((el) =>
      el.shadowRoot!.querySelector('send-icon-sk')
    );

    await (inputHandle as any).type('Hello Puppeteer');
    await (buttonHandle as any).click();

    // Wait for response (which will fail 404 because no backend)
    await new Promise((r) => setTimeout(r, 500));

    const messages = await panelHandle!.evaluate((el) => {
      const msgs = el.shadowRoot!.querySelectorAll('.message');
      return Array.from(msgs).map((m) => m.textContent);
    });

    expect(messages).to.have.length(2);
    expect(messages[0]).to.contain('Hello Puppeteer');
    expect(messages[1]).to.contain('Error');
  });
});
