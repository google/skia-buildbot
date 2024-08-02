import './index';
import '../../../elements-sk/modules/error-toast-sk';
import { setUpExploreDemoEnv } from '../common/test-util';

setUpExploreDemoEnv();

window.perf = {
  commit_range_url: '',
  key_order: ['config'],
  demo: true,
  radius: 7,
  num_shift: 10,
  interesting: 25,
  step_up_only: false,
  display_group_by: true,
  hide_list_of_commits_on_explore: false,
  notifications: 'none',
  fetch_chrome_perf_anomalies: false,
  feedback_url: '',
  chat_url: '',
  help_url_override: '',
  trace_format: 'chrome',
  need_alert_action: false,
  bug_host_url: '',
  git_repo_url: '',
  keys_for_commit_range: [],
};

customElements.whenDefined('explore-multi-sk').then(() => {
  document
    .querySelector('h1')!
    .insertAdjacentElement(
      'afterend',
      document.createElement('explore-multi-sk')
    );
});
