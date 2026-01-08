import './index';
import { assert } from 'chai';
import { ExtraLinksSk } from './extra-links-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { SkPerfConfig } from '../json';

window.perf = {} as SkPerfConfig;

describe('extra-links-sk', () => {
  const newInstance = setUpElementUnderTest<ExtraLinksSk>('extra-links-sk');

  let element: ExtraLinksSk;

  it('renders extra links on connect', async () => {
    window.perf.extra_links = {
      name: 'Page Name',
      title: 'Page Title',
      links: [
        {
          text: 'link 1',
          href: 'https://link1.com/',
          description: 'Description of Link 1',
        },
      ],
    };

    element = newInstance();
    assert.isNotNull(element.querySelector('table'));
    assert.equal(element.querySelector('a')?.textContent, 'link 1');
  });

  it('renders no favorites message if config is empty', async () => {
    window.perf.extra_links = null;
    element = newInstance();
    assert.include(element.textContent || '', 'No links have been configured');
  });
});
