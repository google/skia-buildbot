import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';
import { AnomaliesTableSkPO } from '../anomalies-table-sk/anomalies-table-sk_po';
import { ExploreSimpleSkPO } from '../explore-simple-sk/explore-simple-sk_po';

export class ReportPageSkPO extends PageObject {
  get anomaliesTable(): AnomaliesTableSkPO {
    return this.poBySelector('anomalies-table-sk', AnomaliesTableSkPO);
  }

  get graphContainer(): PageObjectElement {
    return this.bySelector('#graph-container');
  }

  get graphs(): PageObjectElementList {
    return this.bySelectorAll('explore-simple-sk');
  }

  async getGraph(index: number): Promise<ExploreSimpleSkPO> {
    const graphs = await this.graphs;
    return new ExploreSimpleSkPO(await graphs.item(index));
  }

  get commonCommitsDiv(): PageObjectElement {
    return this.bySelector('.common-commits');
  }

  get commonCommitLinks(): PageObjectElementList {
    return this.bySelectorAll('.common-commits a');
  }
}
