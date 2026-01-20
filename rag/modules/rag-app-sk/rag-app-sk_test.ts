import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { setUpElementUnderTest, waitForRender } from '../../../infra-sk/modules/test_util';
import { RagAppSk, Topic } from './rag-app-sk';

describe('rag-app-sk', () => {
  const newInstance = setUpElementUnderTest<RagAppSk>('rag-app-sk');

  let element: RagAppSk;

  beforeEach(async () => {
    fetchMock.get('/config', {
      instance_name: 'Test Instance',
      header_icon_url: 'test-logo.svg',
    });
    element = newInstance();
    await waitForRender(element);
    console.log('Element:', element);
    console.log('ShadowRoot:', element.shadowRoot);
    console.log('Is instance of RagAppSk:', element instanceof RagAppSk);
    console.log('Custom Element defined:', customElements.get('rag-app-sk'));
  });

  afterEach(() => {
    fetchMock.reset();
  });

  it('renders instance name and header icon from config', async () => {
    // Check if the instance name is rendered
    if (!element.shadowRoot) {
      throw new Error('ShadowRoot is null');
    }
    const title = element.shadowRoot.querySelector('h1');
    assert.isNotNull(title);
    assert.equal(title!.textContent, 'Test Instance');

    // Check if the header icon is rendered
    const img = element.shadowRoot!.querySelector('img');
    assert.isNotNull(img);
    assert.equal(img!.getAttribute('src'), 'test-logo.svg');
  });

  it('performs search and displays results', async () => {
    await waitForRender(element);

    const mockTopics: Topic[] = [
      { topicId: 1, topicName: 'Topic 1', summary: 'Summary 1' },
      { topicId: 2, topicName: 'Topic 2', summary: 'Summary 2' },
    ];

    fetchMock.get('/historyrag/v1/topics?query=test&topic_count=10', { topics: mockTopics });

    // Set query and trigger search
    const input = element.shadowRoot!.querySelector('md-outlined-text-field.query-input') as any;
    input.value = 'test';
    input.dispatchEvent(new Event('input'));

    const searchButton = element.shadowRoot!.querySelector('md-filled-button') as HTMLElement;
    searchButton.click();

    await waitForRender(element);

    // Verify search request was made
    assert.isTrue(fetchMock.called('/historyrag/v1/topics?query=test&topic_count=10'));

    // Verify results are displayed
    const topicItems = element.shadowRoot!.querySelectorAll('.topic-item');
    assert.equal(topicItems.length, 2);

    // Verify topic content
    const firstTopicName = topicItems[0].querySelector('.topic-name');
    assert.equal(firstTopicName!.textContent, 'Topic 1');
  });

  it('shows loading spinner when searching', async () => {
    await waitForRender(element);

    // Delay response to check loading state
    fetchMock.get(
      '/historyrag/v1/topics?query=test&topic_count=10',
      new Promise((resolve) => setTimeout(() => resolve({ topics: [] }), 100))
    );

    const input = element.shadowRoot!.querySelector('md-outlined-text-field.query-input') as any;
    input.value = 'test';
    input.dispatchEvent(new Event('input'));

    const searchButton = element.shadowRoot!.querySelector('md-filled-button') as HTMLElement;
    searchButton.click();

    await waitForRender(element);

    // Check for spinner
    const spinnerOverlay = element.shadowRoot!.querySelector('.spinner-overlay');
    assert.isNotNull(spinnerOverlay);
  });
});
