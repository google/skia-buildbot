import './index';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { ImageCompareSk } from './image-compare-sk';
import { ImageCompareSkPO } from './image-compare-sk_po';

describe('image-compare-sk', () => {
  const newInstance = setUpElementUnderTest<ImageCompareSk>('image-compare-sk');

  let imageCompareSk: ImageCompareSk;
  let imageCompareSkPO: ImageCompareSkPO;

  beforeEach(() => {
    imageCompareSk = newInstance();
    imageCompareSkPO = new ImageCompareSkPO(imageCompareSk);
  });

  describe('layout with right and left', () => {
    beforeEach(() => {
      imageCompareSk.left = {
        digest: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        title: 'a digest title',
        detail: 'example.com#aDigest',
      };
      imageCompareSk.right = {
        digest: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        title: 'the other image',
        detail: 'example.com#bDigest',
      };
    });

    it('has three images (left, diff, right) with a zoom button', async () => {
      expect(await imageCompareSkPO.getImageSrcs()).to.deep.equal([
          '/img/images/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png',
          '/img/diffs/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png',
          '/img/images/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png',
      ]);
      expect(await imageCompareSkPO.isZoomBtnVisible()).to.be.true;
    });

    it('captions the images with the respective links', async () => {
      expect(await imageCompareSkPO.getImageCaptionTexts())
          .to.deep.equal(['a digest title', 'the other image']);
      expect(await imageCompareSkPO.getImageCaptionHrefs())
          .to.deep.equal(['example.com#aDigest', 'example.com#bDigest']);
    });

    it('fires events when the zoom dialog is opened and closed', async () => {
      expect(await imageCompareSkPO.isZoomDialogVisible()).to.be.false;
      expect(await imageCompareSkPO.multiZoomSkPO.isEmpty()).to.be.true; // Not rendered at first.
      const openPromise = eventPromise('zoom-dialog-opened');
      await imageCompareSkPO.clickZoomBtn();
      await openPromise;

      // Element should be there now.
      expect(await imageCompareSkPO.isZoomDialogVisible()).to.be.true;
      expect(await imageCompareSkPO.multiZoomSkPO.isEmpty()).to.be.false;

      const closePromise = eventPromise('zoom-dialog-closed');
      await imageCompareSkPO.clickCloseZoomDialogBtn();
      await closePromise;

      // It should be removed from the DOM.
      expect(await imageCompareSkPO.isZoomDialogVisible()).to.be.false;
      expect(await imageCompareSkPO.multiZoomSkPO.isEmpty()).to.be.true;
    });
  });

  describe('layout with just left', () => {
    beforeEach(() => {
      imageCompareSk.left = {
        digest: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        title: 'a digest title',
        detail: 'example.com#aDigest',
      };
    });

    it('has one image and no zoom button', async () => {
      expect(await imageCompareSkPO.getImageSrcs())
          .to.deep.equal(['/img/images/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png']);
      expect(await imageCompareSkPO.isZoomBtnVisible()).to.be.false;
    });
  });
});
