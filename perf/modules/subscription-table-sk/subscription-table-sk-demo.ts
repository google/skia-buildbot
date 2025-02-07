import './index';
import { $$ } from '../../../infra-sk/modules/dom';
import '../../../elements-sk/modules/error-toast-sk';
import { SubscriptionTableSk } from './subscription-table-sk';
import { Alert, Subscription, SerializesToString } from '../json';

const subscription: Subscription = {
  name: 'V8 Jetstream 2',
  revision: '12345',
  bug_labels: ['LabelA', 'LabelB'],
  hotlists: ['HotlistA', 'HotlistB'],
  bug_component: 'ComponentA>SubComponentB',
  bug_priority: 1,
  bug_severity: 2,
  bug_cc_emails: ['user1@google.com', 'user2@google.com'],
  contact_email: 'owner@google.com',
};

const alerts: Alert[] = [
  {
    id_as_string: '5646874153320448',
    display_name: 'A',
    query: 'source_type=image\u0026sub_result=min_ms',
    issue_tracker_component: SerializesToString('720614'),
    alert: '',
    step: 'cohen',
    interesting: 50,
    bug_uri_template: '',
    algo: 'stepfit',
    state: 'ACTIVE',
    owner: 'jcgregorio@google.com',
    step_up_only: false,
    direction: 'BOTH',
    radius: 7,
    k: 0,
    group_by: '',
    sparse: false,
    minimum_num: 0,
    category: ' ',
    action: 'noaction',
  },
  {
    id_as_string: '2',
    display_name: 'B',
    query: 'source_type=image\u0026sub_result=min_ms',
    issue_tracker_component: SerializesToString('720614'),
    alert: '',
    interesting: 50,
    bug_uri_template: '',
    algo: 'stepfit',
    state: 'DELETED',
    owner: 'jcgregorio@google.com',
    step_up_only: false,
    step: 'mannwhitneyu',
    direction: 'BOTH',
    radius: 7,
    k: 0,
    group_by: '',
    sparse: false,
    minimum_num: 0,
    category: 'Stuff',
    action: 'noaction',
  },
];

$$('#populate-tables')?.addEventListener('click', () => {
  document.querySelectorAll<SubscriptionTableSk>('subscription-table-sk').forEach((ele) => {
    ele.load(subscription, alerts);
  });
});

$$('#toggle-tables')?.addEventListener('click', () => {
  document.querySelectorAll<SubscriptionTableSk>('subscription-table-sk').forEach((ele) => {
    ele.toggleAlerts();
  });
});
