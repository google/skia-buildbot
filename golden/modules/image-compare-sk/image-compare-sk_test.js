import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../test_util';

const aDigest = '6246b773851984c726cb2e1cb13510c2';
const bDigest = '99c58c7002073346ff55f446d47d6311';

describe('image-compare-sk', () => {
  const newInstance = setUpElementUnderTest('image-compare-sk');

  let imageCompareSk;
  beforeEach(() => imageCompareSk = newInstance());

  describe('layout with right and left', () => {
    beforeEach(() => {
      imageCompareSk.left = {
        digest: aDigest,
        title: 'a digest title',
        detail: 'example.com#aDigest',
      };
      imageCompareSk.right = {
        digest: bDigest,
        title: 'the other image',
        detail: 'example.com#bDigest',
      };
    });

    it('has three images (left, diff, right) with a zoom button', () => {
      const images = $('img', imageCompareSk);
      expect(images.length).to.equal(3);
      const zBtn = $$('button.zoom_btn');
      expect(zBtn.hidden).to.be.false;
    });

    it('captions the images with the respective links', () => {
      const captions = $('figcaption a', imageCompareSk);
      expect(captions.length).to.equal(2);
      const captionsText = captions.map((c) => c.textContent);
      const captionsHref = captions.map((c) => c.href.substring(c.href.lastIndexOf('/')));

      expect(captionsText).to.contain('a digest title');
      expect(captionsText).to.contain('the other image');

      expect(captionsHref).to.contain('/example.com#aDigest');
      expect(captionsHref).to.contain('/example.com#bDigest');
    });

    it('fires a zoom event when the zoom button is clicked', () => {
      let eventsSeen = 0;
      imageCompareSk.addEventListener('zoom-clicked', (e) => {
        eventsSeen++;
        expect(e.detail).to.deep.equal({
          leftImgUrl: '/img/images/6246b773851984c726cb2e1cb13510c2.png',
          rightImgUrl: '/img/images/99c58c7002073346ff55f446d47d6311.png',
          middleImgUrl: '/img/diffs/6246b773851984c726cb2e1cb13510c2-99c58c7002073346ff55f446d47d6311.png',
          llabel: 'a digest title',
          rlabel: 'the other image',
        });
      });
      $$('button.zoom_btn').click();
      expect(eventsSeen).to.equal(1);
    });
  });

  describe('layout with just left', () => {
    beforeEach(() => {
      imageCompareSk.left = {
        src: aDigest,
        title: 'a digest title',
        detail: 'example.com#aDigest',
      };
    });

    it('has one image and no zoom button', () => {
      const images = $('img', imageCompareSk);
      expect(images.length).to.equal(1);
      const zBtn = $$('button.zoom_btn');
      expect(zBtn.hidden).to.be.true;
    });
  });
});
