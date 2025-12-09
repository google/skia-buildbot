"""This module defines the gold_launcher macro."""

load("@rules_shell//shell:sh_binary.bzl", "sh_binary")
load("//golden/pages:all_gold_pages.bzl", "ALL_GOLD_PAGES")

# Curly braces are escaped ("{" becomes "{{", "}" becomes "}}") because we will format this
# template with https://bazel.build/rules/lib/string#format.
_RUNNER_SCRIPT = """

# Compute the path to the directory with web assets by extracting the parent directory of an
# arbitrary page. This works because all web assets are generated on the same directory (with the
# exception of the favicon, which we will handle later).
byblame_page_assets=($(rootpaths //golden/pages:byblame_dev)) # Expands to byblame.(html,js,css).
web_assets_dir=$$(realpath $$(dirname $${{byblame_page_assets[0]}}))

# Copy the favicon into the web assets directory.
cp $(rootpath //golden/static:favicon.ico) $$web_assets_dir

# Based on //golden/k8s-instances/skia-infra/skia-infra-frontend.json5.
cat > config.json5 <<EOF
{{
  authorized_users: ["google.com"],
  client_secret_file: "$$1",
  frontend: {{
    baseRepoURL: "<inherited from git_repo_url>",
    defaultCorpus: "{default_corpus}",
    title: "{title}",
    customTriagingDisallowedMsg: "{custom_triaging_disallowed_msg}",
  }},
  negatives_max_age: "4320h", // 180 days
  positives_max_age: "720h", // 30 days
  prom_port: ":20000",
  ready_port: ":8000",
  debug_port: ":8001",
  resources_path: "$$web_assets_dir",
  is_public_view: {is_public_view},
  publicly_allowed_params: {publicly_allowed_params},
}}
EOF

# Print contents of file for debugging purposes.
echo "CONTENTS OF FILE $$(pwd)/config.json5:"
cat config.json5
echo

# Based on //golden/k8s-instances/skia-infra/skia-infra.json5.
cat > common_instance_config.json5 <<EOF
{{
  local: true,
  code_review_systems: {code_review_systems},
  gcs_bucket: "{gcs_bucket}",
  git_repo_branch: "{git_repo_branch}",
  git_repo_url: "{git_repo_url}",
  pubsub_project_id: "skia-public",
  site_url: "{site_url}",
  sql_connection: "root@localhost:26235",
  sql_database: "{sql_database}",
  known_hashes_gcs_path: "{known_hashes_gcs_path}",
  window_size: {window_size},
  grouping_param_keys_by_corpus: {grouping_param_keys_by_corpus},
  cache_type: "local",
}}
EOF

# Print contents of file for debugging purposes.
echo "CONTENTS OF FILE $$(pwd)/common_instance_config.json5:"
cat common_instance_config.json5
echo

# Print out all commands from now on for debugging purposes.
set -ex

# Switch to te skia-public GCP project.
gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public
gcloud config set project skia-public

# Port-forward the production CockroachDB instance to port 26235. The default CockroachDB port is
# 26234, so this prevents the forwarded port from clashing with a local running CockroachDB.
#
# This command will be killed on Ctrl+C.
kubectl port-forward gold-cockroachdb-0 26235:26234 &
sleep 5

# Ignore any local running emulators.
FIRESTORE_EMULATOR_HOST=

# Launch the gold_frontend binary.
$(rootpath //golden/cmd/gold_frontend:gold_frontend) \
    --config=config.json5 \
    --common_instance_config=common_instance_config.json5 \
    --log_sql_queries
"""

def gold_launcher(
        name,
        default_corpus,
        title,
        gcs_bucket,
        code_review_systems,
        git_repo_url,
        site_url,
        sql_database,
        known_hashes_gcs_path,
        window_size,
        git_repo_branch = "main",
        is_public_view = False,
        publicly_allowed_params = None,
        grouping_param_keys_by_corpus = None,
        custom_triaging_disallowed_msg = ""):
    """Launches a local gold_frontend instance that talks to a production database.

    This rule is meant for local development and debugging. It reuses the credentials given to the
    kubectl command, which means it might have write access to the production database, so please
    use with caution.

    Args:
        name: Name of the rule.
        default_corpus: Default corpus, e.g. "gm".
        title: Title shown in the Gold UI, e.g. "Skia Gold".
        gcs_bucket: GCS bucket where digests are found, e.g. "skia-infra-gm".
        code_review_systems: A list of dictionaries with keys "id", "flavor", "gerrit_url" and
            "url_template".
        git_repo_url: Git repository URL, e.g. "https://skia.googlesource.com/skia.git".
        site_url: URL of the Gold instance, e.g. "https://gold.skia.org".
        sql_database: Name of the production CockroachDB database, e.g. "skia".
        known_hashes_gcs_path: Path to the known hashes GCS file, e.g.
            "skia-infra-gm/hash_files/gold-prod-hashes.txt".
        window_size: Window size, e.g. 256.
        git_repo_branch: Git repository branch. Optional.
        is_public_view: Whether this is a public mirror of a private instance.
        publicly_allowed_params: A dictionary from corpora to a dictionary from key names to an
            array of allowed values.
        grouping_param_keys_by_corpus: A dictionary where the keys are corpus names and the values
            are a list of param keys, e.g. {"foo": ["a", "b"], "bar": ["c", "d"]}. Optional.
        custom_triaging_disallowed_msg: Custom triaging disallowed message. Optional.
    """
    formatted_runner_script = _RUNNER_SCRIPT.format(
        bazel_target_name = "//%s:%s" % (native.package_name(), name),
        default_corpus = default_corpus,
        title = title,
        custom_triaging_disallowed_msg = custom_triaging_disallowed_msg.replace("\n", r"\n").replace('"', r'\"'),
        grouping_param_keys_by_corpus =
            grouping_param_keys_by_corpus if grouping_param_keys_by_corpus else "null",
        gcs_bucket = gcs_bucket,
        code_review_systems = json.encode(code_review_systems),
        git_repo_url = git_repo_url,
        site_url = site_url,
        sql_database = sql_database,
        known_hashes_gcs_path = known_hashes_gcs_path,
        window_size = window_size,
        git_repo_branch = git_repo_branch,
        is_public_view = "true" if is_public_view else "false",
        publicly_allowed_params = publicly_allowed_params if publicly_allowed_params else "null",
    )

    deps = [
        "//golden/cmd/gold_frontend",
        "//golden/static:favicon.ico",
    ] + ["//golden/pages:%s_dev" % page for page in ALL_GOLD_PAGES]

    native.genrule(
        name = name + "_runner",
        srcs = deps,
        outs = [name + "_runner.sh"],
        cmd = "echo '%s' > $@" % formatted_runner_script,
    )

    sh_binary(
        name = name,
        srcs = [name + "_runner"],
        data = deps,
    )
