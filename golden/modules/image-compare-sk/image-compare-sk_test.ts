import './index';
import { expect } from 'chai';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
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
