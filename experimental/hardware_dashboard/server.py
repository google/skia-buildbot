import http.server
import socketserver
import urllib.request
import urllib.error
import subprocess
import json
import os
import argparse
import re

class ProxyHTTPRequestHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, chromium_dir=None, **kwargs):
        self.chromium_dir = chromium_dir
        super().__init__(*args, **kwargs)

    def end_headers(self):
        self.send_header('Cache-Control', 'no-cache, no-store, must-revalidate')
        self.send_header('Pragma', 'no-cache')
        self.send_header('Expires', '0')
        super().end_headers()


    def get_luci_token(self):
        try:
            result = subprocess.run(['luci-auth', 'token'], capture_output=True, text=True, check=True)
            return result.stdout.strip()
        except subprocess.CalledProcessError as e:
            print(f"Error getting luci-auth token: {e}")
            return None
        except FileNotFoundError:
            print("luci-auth command not found. Please ensure it is installed and in your PATH.")
            return None

    def do_GET(self):
        parsed_path = urllib.parse.urlparse(self.path)

        if parsed_path.path == '/api/builders':
            self.handle_api_builders()
        elif parsed_path.path == '/api/trigger_mapping':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.send_header('Cache-Control', 'no-cache, no-store, must-revalidate')
            self.end_headers()
            self.wfile.write(json.dumps(self.get_trigger_mapping()).encode('utf-8'))
        elif parsed_path.path == '/api/gn_args':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.send_header('Cache-Control', 'no-cache, no-store, must-revalidate')
            self.end_headers()
            self.wfile.write(json.dumps(self.get_gn_args_mapping()).encode('utf-8'))
        elif parsed_path.path == '/api/swarming_dimensions':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.send_header('Cache-Control', 'no-cache, no-store, must-revalidate')
            self.end_headers()
            self.wfile.write(json.dumps(self.get_swarming_dimensions_mapping()).encode('utf-8'))
        elif parsed_path.path == '/api/benchmarks':
            self.handle_api_benchmarks()
        else:
            super().do_GET()

    def do_POST(self):
        if self.path == '/api/swarming_bots':
            self.handle_api_swarming_bots()
        else:
            self.send_error(404)

    def get_trigger_mapping(self):
        import os, re, subprocess, json, tempfile, tarfile
        mapping = {}

        # Method 1: Try gob-curl against Gitiles API (+archive)
        try:
            archive_url = "https://chrome-internal.googlesource.com/chrome/src-internal/+archive/main/infra/config/subprojects/chrome/ci.tar.gz"
            with tempfile.NamedTemporaryFile(suffix='.tar.gz') as temp_archive:
                subprocess.run(
                    f'gob-curl "{archive_url}" > "{temp_archive.name}"',
                    shell=True, check=True
                )
                with tarfile.open(temp_archive.name, 'r:gz') as tar:
                    for member in tar.getmembers():
                        if member.name.endswith('.star'):
                            f = tar.extractfile(member)
                            if f:
                                content = f.read().decode('utf-8')
                                current_name = None
                                for line in content.split('\n'):
                                    name_match = re.search(r'name\s*=\s*"([^"]+)"', line)
                                    if name_match:
                                        current_name = name_match.group(1)

                                    trigger = None
                                    trigger_match = re.search(r'triggered_by\s*=\s*\["([^"]+)"\]', line)
                                    parent_match = re.search(r'parent\s*=\s*"([^"]+)"', line)

                                    if trigger_match:
                                        trigger = trigger_match.group(1)
                                    elif parent_match:
                                        trigger = parent_match.group(1)

                                    if trigger and current_name:
                                        if trigger.startswith('ci/'):
                                            trigger = trigger[3:]
                                        mapping[current_name] = trigger

            if mapping:
                return mapping

        except Exception as e:
            print(f"gob-curl fetch failed: {e}. Falling back to local chromium-dir...")

        # Method 2: Fallback to local chromium_dir
        if not self.chromium_dir:
            return mapping

        config_dir = os.path.join(self.chromium_dir, "internal/infra/config/subprojects/chrome/ci")
        if not os.path.exists(config_dir):
            print(f"Warning: Config directory not found at {config_dir}")
            return mapping

        star_files = [f for f in os.listdir(config_dir) if f.endswith('.star')]
        for filename in star_files:
            filepath = os.path.join(config_dir, filename)
            try:
                with open(filepath, 'r') as f:
                    content = f.read()
                current_name = None
                for line in content.split('\n'):
                    name_match = re.search(r'name\s*=\s*"([^"]+)"', line)
                    if name_match:
                        current_name = name_match.group(1)

                    trigger = None
                    trigger_match = re.search(r'triggered_by\s*=\s*\["([^"]+)"\]', line)
                    parent_match = re.search(r'parent\s*=\s*"([^"]+)"', line)

                    if trigger_match:
                        trigger = trigger_match.group(1)
                    elif parent_match:
                        trigger = parent_match.group(1)

                    if trigger and current_name:
                        if trigger.startswith('ci/'):
                            trigger = trigger[3:]
                        mapping[current_name] = trigger
            except Exception:
                pass
        return mapping

    def get_gn_args_mapping(self):
        import urllib.request, base64, ast
        url = "https://chromium.googlesource.com/chromium/src/+/main/tools/mb/mb_config.pyl?format=TEXT"
        try:
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req) as response:
                content = base64.b64decode(response.read()).decode("utf-8")
            pyl = ast.literal_eval(content)

            gn_args_map = {}
            for group, builders in pyl.get('builder_groups', {}).items():
                if group != 'chromium.perf':
                    continue
                for builder, config_name in builders.items():
                    if isinstance(config_name, dict):
                        # Handle case where builder maps to a nested dict instead of string
                        for k, v in config_name.items():
                             if k == "chromium.perf": config_name = v
                    if not isinstance(config_name, str):
                        continue

                    mixins = pyl.get('configs', {}).get(config_name, [])

                    args_parts = []
                    for mixin in mixins:
                        mixin_dict = pyl.get('mixins', {}).get(mixin, {})
                        if 'gn_args' in mixin_dict:
                            args_parts.append(mixin_dict['gn_args'])

                    if args_parts:
                        gn_args_map[builder] = " ".join(args_parts)

            return gn_args_map
        except Exception as e:
            print(f"Error fetching mb_config.pyl: {e}")
            return {}

    def get_swarming_dimensions_mapping(self):
        import urllib.request, json, base64
        url = "https://chromium.googlesource.com/chromium/src/+/refs/heads/main/testing/buildbot/chromium.perf.json?format=TEXT"
        try:
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req) as response:
                content = base64.b64decode(response.read()).decode("utf-8")
            data = json.loads(content)

            dims_map = {}
            for builder, config in data.items():
                if builder.startswith("AAAA"): continue

                scripts = config.get("isolated_scripts", [])
                if not scripts: continue

                swarming = scripts[0].get("swarming", {})
                dimensions = swarming.get("dimensions", {})
                dims_map[builder] = dimensions

            return dims_map
        except Exception as e:
            print(f"Error fetching chromium.perf.json: {e}")
            return {}

    def handle_api_benchmarks(self):
        import tarfile
        import io
        import urllib.request
        import glob

        benchmarks_mapping = {}

        if self.chromium_dir:
            schedule_dir = os.path.join(self.chromium_dir, "tools", "perf", "core", "schedule")
            if os.path.exists(schedule_dir):
                for filepath in glob.glob(os.path.join(schedule_dir, "*.csv")):
                    benchmark_name = os.path.basename(filepath).replace(".csv", "")
                    try:
                        with open(filepath, 'r') as f:
                            lines = f.readlines()
                            for line in lines[1:]:
                                parts = line.strip().split(',')
                                if len(parts) >= 1:
                                    bot = parts[0]
                                    if bot not in benchmarks_mapping:
                                        benchmarks_mapping[bot] = []
                                    benchmarks_mapping[bot].append(benchmark_name)
                    except Exception as e:
                        pass

        if not benchmarks_mapping:
            url = "https://chromium.googlesource.com/chromium/src/+archive/refs/heads/main/tools/perf/core/schedule.tar.gz"
            try:
                req = urllib.request.Request(url)
                resp = urllib.request.urlopen(req).read()
                with tarfile.open(fileobj=io.BytesIO(resp), mode="r:gz") as tar:
                    for member in tar.getmembers():
                        if member.name.endswith(".csv"):
                            benchmark_name = member.name.replace(".csv", "")
                            f = tar.extractfile(member)
                            if f:
                                content = f.read().decode('utf-8')
                                lines = content.splitlines()
                                for line in lines[1:]:
                                    parts = line.strip().split(',')
                                    if len(parts) >= 1:
                                        bot = parts[0]
                                        if bot not in benchmarks_mapping:
                                            benchmarks_mapping[bot] = []
                                        benchmarks_mapping[bot].append(benchmark_name)
            except Exception as e:
                print("Failed to fetch schedule tarball:", e)

        for bot in benchmarks_mapping:
            benchmarks_mapping[bot] = sorted(list(set(benchmarks_mapping[bot])))

        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(benchmarks_mapping).encode('utf-8'))

    def handle_api_swarming_bots(self):
        content_length = int(self.headers['Content-Length'])
        post_data = self.rfile.read(content_length)
        req_data = json.loads(post_data)
        dimensions_dict = req_data.get('dimensions', {})
        swarming_server = req_data.get('server', 'https://chrome-swarming.appspot.com')

        token = self.get_luci_token()
        if not token:
            self.send_error(500, "Failed to get luci-auth token")
            return

        dimensions_list = [{"key": k, "value": str(v)} for k, v in dimensions_dict.items()]

        url = f"{swarming_server}/prpc/swarming.v2.Bots/ListBots"
        payload = json.dumps({
            "dimensions": dimensions_list
        }).encode("utf-8")

        req = urllib.request.Request(url, data=payload, method="POST")
        req.add_header("Content-Type", "application/json")
        req.add_header("Accept", "application/json")
        req.add_header("Authorization", f"Bearer {token}")

        try:
            with urllib.request.urlopen(req) as response:
                resp = response.read().decode("utf-8")
                if resp.startswith(")]}'\n"):
                    resp = resp[5:]

                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.send_header('Cache-Control', 'no-cache, no-store, must-revalidate')
                self.send_header('Pragma', 'no-cache')
                self.send_header('Expires', '0')
                self.end_headers()
                self.wfile.write(resp.encode('utf-8'))
        except Exception as e:
            print(f"Error fetching ListBots: {e}")
            self.send_error(500, str(e))

    def handle_api_builders(self):
        token = self.get_luci_token()
        if not token:
            self.send_response(500)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"error": "Failed to get luci-auth token. Please run bb auth-login."}')
            return

        url = 'https://cr-buildbucket.appspot.com/prpc/buildbucket.v2.Builders/ListBuilders'
        payload = json.dumps({
            "project": "chrome",
            "bucket": "ci",
            "pageSize": 1000
        }).encode('utf-8')

        req = urllib.request.Request(url, data=payload, method='POST')
        req.add_header('Content-Type', 'application/json')
        req.add_header('Accept', 'application/json')
        req.add_header('Authorization', f'Bearer {token}')

        try:
            with urllib.request.urlopen(req) as response:
                resp_data = response.read().decode('utf-8')

                # Strip pRPC prefix
                if resp_data.startswith(")]}'\n"):
                    resp_data = resp_data[5:]

                json_data = json.loads(resp_data)

                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps(json_data).encode('utf-8'))

        except urllib.error.HTTPError as e:
            self.send_response(e.code)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            err_msg = e.read().decode('utf-8')
            self.wfile.write(json.dumps({"error": f"Buildbucket API Error: {err_msg}"}).encode('utf-8'))
        except Exception as e:
            self.send_response(500)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({"error": str(e)}).encode('utf-8'))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description="Hardware Dashboard Proxy Server")
    parser.add_argument("--port", type=int, default=8081, help="Port to run the server on")
    parser.add_argument("--chromium-dir", type=str, help="Path to chromium/src directory for parsing configs")
    args = parser.parse_args()

    # Create a handler factory with the chromium_dir baked in
    def handler_factory(*h_args, **h_kwargs):
        return ProxyHTTPRequestHandler(*h_args, chromium_dir=args.chromium_dir, **h_kwargs)

    socketserver.TCPServer.allow_reuse_address = True
    with socketserver.TCPServer(("", args.port), handler_factory) as httpd:
        print(f"Serving at port {args.port}. Stop the old Python server and run this instead!")
        if not args.chromium_dir:
            print("Note: --chromium-dir was not provided. Builder mapping will fallback to unmapped.")
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            pass
