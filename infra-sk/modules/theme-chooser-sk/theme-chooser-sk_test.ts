import { expect } from 'chai';
import { setUpElementUnderTest, eventPromise } from '../test_util';
import { ThemeChooserSk, ThemeChooserSkEventDetail, DARKMODE_CLASS } from './theme-chooser-sk';

describe('theme-chooser-sk', () => {
  const newInstance = setUpElementUnderTest<ThemeChooserSk>('theme-chooser-sk');

  let themeChooserSk: ThemeChooserSk;

  beforeEach(() => {
    themeChooserSk = newInstance();
  });

  it('generates event and toggles from darkmode when clicked', async () => {
    themeChooserSk.darkmode = true;
    const event = eventPromise<CustomEvent<ThemeChooserSkEventDetail>>('theme-chooser-toggle');
    themeChooserSk.click();
    expect((await event).detail.darkmode).to.be.false;
    expect(document.body.classList.contains(DARKMODE_CLASS)).to.be.false;
  });

  it('generates event and toggles to darkmode when clicked', async () => {
    themeChooserSk.darkmode = false;
    const event = eventPromise<CustomEvent<ThemeChooserSkEventDetail>>('theme-chooser-toggle');
    themeChooserSk.click();
    expect((await event).detail.darkmode).to.be.true;
    expect(document.body.classList.contains(DARKMODE_CLASS)).to.be.true;
  });
});
