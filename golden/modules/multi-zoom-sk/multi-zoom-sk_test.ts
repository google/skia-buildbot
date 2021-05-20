import './index';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { diff16x16, left16x16, right16x16 } from './test_data';
import { MultiZoomSk } from './multi-zoom-sk';
import { MultiZoomSkPO } from './multi-zoom-sk_po';
import { expect } from 'chai';

describe('multi-zoom-sk', () => {
  const newInstance = setUpElementUnderTest<MultiZoomSk>('multi-zoom-sk');

  let multiZoomSk: MultiZoomSk;
  let multiZoomSkPO: MultiZoomSkPO;

  beforeEach(async () => {
    multiZoomSk = newInstance();
    multiZoomSkPO = new MultiZoomSkPO(multiZoomSk);

    const event = eventPromise('sources-loaded');
    multiZoomSk.details = {
      leftImageSrc: left16x16,
      diffImageSrc: diff16x16,
      rightImageSrc: right16x16,
      leftLabel: 'left16x16',
      rightLabel: 'right16x16',
    };
    // Even though the image is supplied via base64 encoding and no network traffic is required,
    // the browser asynchronously decodes the image, and then calls the load callback. Thus, we need
    // to wait for the appropriate event.
    await event;
  });

  it('has loaded pixel data for all three images', () => {
    // This is a private property.
    const loadedImageData = (multiZoomSk as any).loadedImageData as ImageData[];

    expect(loadedImageData.length).to.equal(3);
    // These are 16 x 16 images and the ImageData objects should reflect this.
    expect(loadedImageData[0].height).to.equal(16);
    expect(loadedImageData[0].width).to.equal(16);
    expect(loadedImageData[1].height).to.equal(16);
    expect(loadedImageData[1].width).to.equal(16);
    expect(loadedImageData[2].height).to.equal(16);
    expect(loadedImageData[2].width).to.equal(16);
  });

  it('uses u and y to go between biggest pixel diffs', async () => {
    // default values
    expect(multiZoomSk.x).to.equal(0);
    expect(multiZoomSk.y).to.equal(0);
    expect(await multiZoomSkPO.getCoordinate()).to.equal('(0, 0)');
    expect(await multiZoomSkPO.isNthDiffVisible()).to.be.false;

    await multiZoomSkPO.sendKeypress('u');
    expect(multiZoomSk.x).to.equal(12);
    expect(multiZoomSk.y).to.equal(13);
    expect(await multiZoomSkPO.getCoordinate()).to.equal('(12, 13)');

    expect(await multiZoomSkPO.getNthDiff()).to.equal('1st biggest pixel diff (out of 32)');

    await multiZoomSkPO.sendKeypress('u');
    expect(multiZoomSk.x).to.equal(12);
    expect(multiZoomSk.y).to.equal(14);
    expect(await multiZoomSkPO.getCoordinate()).to.equal('(12, 14)');

    expect(await multiZoomSkPO.getNthDiff()).to.equal('2nd biggest pixel diff (out of 32)');

    await multiZoomSkPO.sendKeypress('y');
    expect(multiZoomSk.x).to.equal(12);
    expect(multiZoomSk.y).to.equal(13)
    expect(await multiZoomSkPO.getCoordinate()).to.equal('(12, 13)');

    expect(await multiZoomSkPO.getNthDiff()).to.equal('1st biggest pixel diff (out of 32)');

    // Already at the beginning - this won't move anywhere.
    await multiZoomSkPO.sendKeypress('y');
    expect(multiZoomSk.x).to.equal(12);
    expect(multiZoomSk.y).to.equal(13);
    expect(await multiZoomSkPO.getCoordinate()).to.equal('(12, 13)');
  });

  it('provides pixel diff information for the current pixel once calculated', async () => {
    multiZoomSk.x = 12;
    multiZoomSk.y = 0;

    // We know there's a diff, but we haven't calculated all the diffs yet (it's done on demand
    // the first time the user clicks 'u')
    expect(await multiZoomSkPO.getLeftPixel()).to.equal('rgba(255, 10, 10, 128) #FF0A0A80');
    expect(await multiZoomSkPO.getDiffPixel()).to.equal('rgba(0, 0, 0, 76)');
    expect(await multiZoomSkPO.getRightPixel()).to.equal('rgba(255, 10, 10, 204) #FF0A0ACC');
    expect(await multiZoomSkPO.isNthDiffVisible()).to.be.false;

    // calculate the diffs.
    await multiZoomSkPO.sendKeypress('u');
    // Now when we move back to that pixel, we will see where this pixel diff ranks.
    multiZoomSk.x = 12;
    multiZoomSk.y = 0;

    expect(await multiZoomSkPO.getDiffPixel()).to.equal('rgba(0, 0, 0, 76)');
    expect(await multiZoomSkPO.getNthDiff()).to.equal('13th biggest pixel diff (out of 32)');
  });

  it('uses z and a to zoom in and out', async () => {
    // default value
    expect(multiZoomSk.zoomLevel).to.equal(8);

    // Zoom should increase by a factor of 2.
    await multiZoomSkPO.sendKeypress('z');
    expect(multiZoomSk.zoomLevel).to.equal(16);

    // and decrease by a factor of 2.
    await multiZoomSkPO.sendKeypress('a');
    expect(multiZoomSk.zoomLevel).to.equal(8);

    await multiZoomSkPO.sendKeypress('a'); // 4
    await multiZoomSkPO.sendKeypress('a'); // 2
    await multiZoomSkPO.sendKeypress('a'); // 1
    await multiZoomSkPO.sendKeypress('a'); // clamp at 1
    await multiZoomSkPO.sendKeypress('a'); // clamp at 1
    expect(multiZoomSk.zoomLevel).to.equal(1);
  });

  it('uses g to toggle the grid', async () => {
    // default value
    expect(multiZoomSk.showGrid).to.equal(false);

    await multiZoomSkPO.sendKeypress('g');
    expect(multiZoomSk.showGrid).to.equal(true);

    await multiZoomSkPO.sendKeypress('g');
    expect(multiZoomSk.showGrid).to.equal(false);
  });

  it('uses m to manually go through the images selected by the checkboxes', async () => {
    // default values
    expect(multiZoomSk.cyclingView).to.equal(true);
    expect(await multiZoomSkPO.isLeftCheckboxChecked()).to.be.true;
    expect(await multiZoomSkPO.isDiffCheckboxChecked()).to.be.false;
    expect(await multiZoomSkPO.isRightCheckboxChecked()).to.be.true;

    // By default, only rotates through the left and right images (index 0 and 2).
    expect(await multiZoomSkPO.getDisplayedImage()).to.equal('left');
    await multiZoomSkPO.sendKeypress('m');
    expect(multiZoomSk.cyclingView).to.equal(false);
    expect(await multiZoomSkPO.getDisplayedImage()).to.equal('right');

    await multiZoomSkPO.sendKeypress('m');
    expect(multiZoomSk.cyclingView).to.equal(false);
    expect(await multiZoomSkPO.getDisplayedImage()).to.equal('left');

    await multiZoomSkPO.clickDiffCheckbox();
    expect(await multiZoomSkPO.isLeftCheckboxChecked()).to.be.true;
    expect(await multiZoomSkPO.isDiffCheckboxChecked()).to.be.true;
    expect(await multiZoomSkPO.isRightCheckboxChecked()).to.be.true;

    await multiZoomSkPO.sendKeypress('m');
    expect(await multiZoomSkPO.isLeftDisplayed())
    expect(await multiZoomSkPO.getDisplayedImage()).to.equal('diff');

    await multiZoomSkPO.sendKeypress('m');
    expect(await multiZoomSkPO.getDisplayedImage()).to.equal('right');

    await multiZoomSkPO.sendKeypress('m');
    expect(await multiZoomSkPO.getDisplayedImage()).to.equal('left');

    // Uncheck the left and diff box (even though the left image is currently being displayed).
    await multiZoomSkPO.clickLeftCheckbox();
    await multiZoomSkPO.clickDiffCheckbox();
    expect(await multiZoomSkPO.isLeftCheckboxChecked()).to.be.false;
    expect(await multiZoomSkPO.isDiffCheckboxChecked()).to.be.false;
    expect(await multiZoomSkPO.isRightCheckboxChecked()).to.be.true;

    // It should snap to the right image and not move, since there are no other images selected.
    await multiZoomSkPO.sendKeypress('m');
    expect(await multiZoomSkPO.getDisplayedImage()).to.equal('right');
    await multiZoomSkPO.sendKeypress('m');
    expect(await multiZoomSkPO.getDisplayedImage()).to.equal('right');
  });

  it('uses h, j, k, and l to move the cursor', async () => {
    // default value
    expect(multiZoomSk.x).to.equal(0);
    expect(multiZoomSk.y).to.equal(0);

    await multiZoomSkPO.sendKeypress('j'); // down
    expect(multiZoomSk.x).to.equal(0);
    expect(multiZoomSk.y).to.equal(1);

    await multiZoomSkPO.sendKeypress('l'); // right
    expect(multiZoomSk.x).to.equal(1);
    expect(multiZoomSk.y).to.equal(1);

    await multiZoomSkPO.sendKeypress('k'); // up
    expect(multiZoomSk.x).to.equal(1);
    expect(multiZoomSk.y).to.equal(0);

    await multiZoomSkPO.sendKeypress('h'); // left
    expect(multiZoomSk.x).to.equal(0);
    expect(multiZoomSk.y).to.equal(0);
  });
});
