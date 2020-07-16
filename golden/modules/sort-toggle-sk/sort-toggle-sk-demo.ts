import './index';
import {html, render} from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import {repeat} from "lit-html/directives/repeat";
import {SortToggleSk} from "./sort-toggle-sk";

interface DemoSortable {
  name: string;
  cost: number;
  weight: number;
}

const data = [
  {
    name: 'bravo',
    cost: 10,
    weight: 16,
  },
  {
    name: 'alfa',
    cost: 8,
    weight: 13,
  },
  {
    name: 'charlie',
    cost: 4,
    weight: 200,
  },
  {
    name: 'delta',
    cost: 2,
    weight: 4,
  }
];


const rowTemplate = (row: DemoSortable) => html`
<tr>
  <td>${row.name}</td>
  <td>${row.cost}</td>
  <td>${row.weight}</td>
</tr>
`;

// lit-html (or maybe html in general) doesn't like sort-toggle-sk to go inside the table.
const usingMap = html`
<sort-toggle-sk .data=${data} @sort-changed=${renderTemplates}>
  <table>
     <thead>
         <tr>
          <th data-key=name data-default=up data-sort-type=alpha>Item</th>
          <th data-key=cost>Cost</th>
          <th data-key=weight>Weight</th>
        </tr>
    </thead>
    <tbody>
      <!-- map is generally faster than repeat when the rowTemplate is small, but its
           for this demo, map wasn't working quite right with data being a global.-->
      ${repeat(data, (row) => row.name, rowTemplate)}
    </tbody>
  </table>
</sort-toggle-sk>`;


function renderTemplates() {
  render(usingMap, $$('#container')!);
}

renderTemplates();
