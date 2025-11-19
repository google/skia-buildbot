import './index';
import '../../../infra-sk/modules/theme-chooser-sk';

window.perf = Object.assign(
  {
    instance_url: '',
    instance_name: 'chrome-perf-demo',
    header_image_url: '',
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
    fetch_anomalies_from_sql: false,
    feedback_url: '',
    chat_url: '',
    help_url_override: '',
    trace_format: '',
    need_alert_action: false,
    bug_host_url: '',
    git_repo_url: 'https://skia.googlesource.com/buildbot',
    keys_for_commit_range: [],
    keys_for_useful_links: [],
    skip_commit_detail_display: false,
    image_tag: 'fake-tag@tag:git-123456789',
    remove_default_stat_value: false,
    enable_skia_bridge_aggregation: false,
    show_json_file_display: false,
    always_show_commit_info: false,
    show_triage_link: true,
    show_bisect_btn: true,
    app_version: '33f07b6a266149a5355120d8b082880b2e98b73e',
    enable_v2_ui: false,
  },
  window.perf || {}
);

document.querySelector('.component-goes-here')!.innerHTML = `
<perf-scaffold-sk>
  <div>
    Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy
    eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam
    voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet
    clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit
    amet.
  </div>
  <div>
    Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy
    eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam
    voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet
    clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit
    amet.
  </div>
  <div id="sidebar_help">Helpful stuff goes here.</div>
</perf-scaffold-sk>
`;
