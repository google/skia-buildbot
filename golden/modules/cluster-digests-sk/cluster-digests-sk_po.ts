import {PageObject} from '../../../infra-sk/modules/page_object/page_object';
import {PageObjectElement, PageObjectElementList} from '../../../infra-sk/modules/page_object/page_object_element';
import {Digest} from '../rpc_types';

export class ClusterDigestsSkPO extends PageObject {
  private get nodes(): PageObjectElementList {
    return this.bySelectorAll('circle.node');
  }

  private get selectedNodes(): PageObjectElementList {
    return this.bySelectorAll('circle.node.selected');
  }

  private get background(): PageObjectElement {
    return this.bySelector('svg');
  }

  async clickNode(digest: Digest) {
    await this.dispatchClickEventOnNode(digest, /* shiftKey= */ false);
  }

  async shiftClickNode(digest: Digest) {
    await this.dispatchClickEventOnNode(digest, /* shiftKey= */ true);
  }

  private async dispatchClickEventOnNode(digest: Digest, shiftKey: boolean) {
    // SVGElements (https://developer.mozilla.org/en-US/docs/Web/API/SVGElement) do not have a
    // .click() method, like HTMLElements do. Thus, we have no alternative but to simulate a click
    // via a fake MouseEvent. This ensures compatibility with both Karma and Puppeteer tests.
    const node =
        await this.nodes.find(async (node) => (await node.getAttribute('data-digest')) === digest);
    await node!.applyFnToDOMNode((el, shiftKey) => {
      el.dispatchEvent(new MouseEvent('click', {shiftKey: shiftKey as boolean}));
    }, shiftKey);
  }

  async clickBackground() {
    await this.background.click();
  }

  async getSelection(): Promise<Digest[]> {
    return this.selectedNodes.map(async (circle) => (await circle.getAttribute('data-digest'))!);
  }
}
