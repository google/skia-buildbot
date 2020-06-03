import './index';
import { $$ } from 'common-sk/modules/dom';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { diff16x16, left16x16, right16x16 } from './test_data';

describe('multi-zoom-sk', () => {
  const newInstance = setUpElementUnderTest('multi-zoom-sk');

  let multiZoomSk;
  beforeEach(async () => {
    multiZoomSk = newInstance();
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
    expect(multiZoomSk._loadedImageData.length).to.equal(3);
    // These are 16 x 16 images and the ImageData objects should reflect this.
    expect(multiZoomSk._loadedImageData[0].height).to.equal(16);
    expect(multiZoomSk._loadedImageData[0].width).to.equal(16);
    expect(multiZoomSk._loadedImageData[1].height).to.equal(16);
    expect(multiZoomSk._loadedImageData[1].width).to.equal(16);
    expect(multiZoomSk._loadedImageData[2].height).to.equal(16);
    expect(multiZoomSk._loadedImageData[2].width).to.equal(16);
  });

  it('uses u and y to go between biggest pixel diffs', () => {
    // default values
    expect(multiZoomSk._x).to.equal(0);
    expect(multiZoomSk._y).to.equal(0);
    expect($$('.nth_diff', multiZoomSk)).to.be.null;

    sendKeyboardEvent('u');
    expect(multiZoomSk._x).to.equal(12);
    expect(multiZoomSk._y).to.equal(13);

    expect($$('.nth_diff', multiZoomSk).innerText).to.equal('1st biggest pixel diff (out of 32)');

    sendKeyboardEvent('u');
    expect(multiZoomSk._x).to.equal(12);
    expect(multiZoomSk._y).to.equal(14);

    expect($$('.nth_diff', multiZoomSk).innerText).to.equal('2nd biggest pixel diff (out of 32)');

    sendKeyboardEvent('y');
    expect(multiZoomSk._x).to.equal(12);
    expect(multiZoomSk._y).to.equal(13);

    expect($$('.nth_diff', multiZoomSk).innerText).to.equal('1st biggest pixel diff (out of 32)');

    // Already at the beginning - this won't move anywhere.
    sendKeyboardEvent('y');
    expect(multiZoomSk._x).to.equal(12);
    expect(multiZoomSk._y).to.equal(13);
  });

  it('provides pixel diff information for the current pixel once calculated', () => {
    multiZoomSk._x = 12;
    multiZoomSk._y = 0;
    multiZoomSk._render();

    // We know there's a diff, but we haven't calculated all the diffs yet (it's done on demand
    // the first time the user clicks 'u')
    expect($$('.stats .diff', multiZoomSk).innerText).to.equal('rgba(0, 0, 0, 76)');
    expect($$('.nth_diff', multiZoomSk)).to.be.null;

    // calculate the diffs.
    sendKeyboardEvent('u');
    // Now when we move back to that pixel, we will see where this pixel diff ranks.
    multiZoomSk._x = 12;
    multiZoomSk._y = 0;
    multiZoomSk._render();

    expect($$('.stats .diff', multiZoomSk).innerText).to.equal('rgba(0, 0, 0, 76)');
    expect($$('.nth_diff', multiZoomSk).innerText).to.equal('13th biggest pixel diff (out of 32)');
  });

  it('uses z and a to zoom in and out', () => {
    // default value
    expect(multiZoomSk._zoomLevel).to.equal(8);

    // Zoom should increase by a factor of 2.
    sendKeyboardEvent('z');
    expect(multiZoomSk._zoomLevel).to.equal(16);

    // and decrease by a factor of 2.
    sendKeyboardEvent('a');
    expect(multiZoomSk._zoomLevel).to.equal(8);

    sendKeyboardEvent('a'); // 4
    sendKeyboardEvent('a'); // 2
    sendKeyboardEvent('a'); // 1
    sendKeyboardEvent('a'); // clamp at 1
    sendKeyboardEvent('a'); // clamp at 1
    expect(multiZoomSk._zoomLevel).to.equal(1);
  });

  it('uses g to toggle the grid', () => {
    // default value
    expect(multiZoomSk._showGrid).to.equal(false);

    sendKeyboardEvent('g');
    expect(multiZoomSk._showGrid).to.equal(true);

    sendKeyboardEvent('g');
    expect(multiZoomSk._showGrid).to.equal(false);
  });

  it('uses m to manually go through the images selected by the checkboxes', () => {
    // default values
    expect(multiZoomSk._cyclingView).to.equal(true);
    expect(multiZoomSk._cycleThrough).to.deep.equal([true, false, true]);

    // by default, only rotates through the left and right images (index 0 and 2).
    multiZoomSk._zoomedIndex = 0;
    sendKeyboardEvent('m');
    expect(multiZoomSk._cyclingView).to.equal(false);
    expect(multiZoomSk._zoomedIndex).to.equal(2);

    sendKeyboardEvent('m');
    expect(multiZoomSk._cyclingView).to.equal(false);
    expect(multiZoomSk._zoomedIndex).to.equal(0);

    // idx_1 is the diff check box
    $$('checkbox-sk.idx_1 input', multiZoomSk).click();
    expect(multiZoomSk._cycleThrough).to.deep.equal([true, true, true]);

    sendKeyboardEvent('m');
    expect(multiZoomSk._zoomedIndex).to.equal(1);

    sendKeyboardEvent('m');
    expect(multiZoomSk._zoomedIndex).to.equal(2);

    sendKeyboardEvent('m');
    expect(multiZoomSk._zoomedIndex).to.equal(0);

    // Un check the left and diff box (even though the left image is currently being displayed
    $$('checkbox-sk.idx_0 input', multiZoomSk).click();
    $$('checkbox-sk.idx_1 input', multiZoomSk).click();
    expect(multiZoomSk._cycleThrough).to.deep.equal([false, false, true]);

    // It should snap to the right image and not move, since there are no other images selected.
    sendKeyboardEvent('m');
    expect(multiZoomSk._zoomedIndex).to.equal(2);
    sendKeyboardEvent('m');
    expect(multiZoomSk._zoomedIndex).to.equal(2);
  });

  it('uses h, j, k, and l to move the cursor', () => {
    // default value
    expect(multiZoomSk._x).to.equal(0);
    expect(multiZoomSk._y).to.equal(0);

    sendKeyboardEvent('j'); // down
    expect(multiZoomSk._x).to.equal(0);
    expect(multiZoomSk._y).to.equal(1);

    sendKeyboardEvent('l'); // right
    expect(multiZoomSk._x).to.equal(1);
    expect(multiZoomSk._y).to.equal(1);

    sendKeyboardEvent('k'); // up
    expect(multiZoomSk._x).to.equal(1);
    expect(multiZoomSk._y).to.equal(0);

    sendKeyboardEvent('h'); // left
    expect(multiZoomSk._x).to.equal(0);
    expect(multiZoomSk._y).to.equal(0);
  });
});

function sendKeyboardEvent(key) {
  document.dispatchEvent(new KeyboardEvent('keydown', {
    key: key,
  }));
}
