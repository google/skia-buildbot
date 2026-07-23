# Skia Perf

**Skia Perf** (also known as the **Performance Dashboard**) is a distributed
system for monitoring and analyzing performance
benchmarking metrics produced across Continuous Integration (CI) and postsubmit
testing workflows. It reads performance data from databases and serves
interactive dashboards to highlight how a commit impacts performance, allowing
for easy exploration and annotation.

While originally developed by the Skia project to track 2D graphics performance
and nanobenchmarks, Perf serves as the standard performance analysis dashboard
across multiple open-source projects. The product is particularly useful for:

- **Chrome Developers** who are interested in performance regressions.
- **Marketing Teams** who need to track, visualize, and communicate product's
  performance enhancements.

---

# Table of Contents

- [About](#about)
  - [Core Capabilities](#core-capabilities)
  - [History](#history)
  - [Terminology](#terminology)
  - [Production Instances](#production-instances)
- [User guide](./docs/user_guide.md)
- [Developer guide](./docs/developer_guide.md)
- [Help](./docs/help.md)
- [Other Documentation](#other-documentation)

---

# About

## Core Capabilities

- **Continuous Metric Ingestion**: Stores time-series data from automated benchmark suites run across various platforms, architectures, and hardware configurations.
- **Anomaly & Regression Detection**: Algorithms monitor incoming telemetry for significant changepoints and regressions, automatically filing alerts and issue reports.
- **Interactive Data Exploration**: Web interfaces for plotting metrics over time, comparing commits, viewing blame lists, and exploring cluster trends.

## History

The current Perf infrastructure in this repo was originally developed for Skia.
In 2023, a project began to unify it with Chrome's performance tooling,
replacing a legacy Python-based system, called Chromeperf. This unification
effort involves consolidating features from both platforms onto this modern Go
and TypeScript stack, with the goal of eventually deprecating the older system.

## Terminology

- **Benchmark**: A top-level test name.
- **Test**: A specific test case within a benchmark.
- **Subtest**: A further breakdown of a test.
- **Bot**: The device or machine that runs the tests.
- **Trace**: A single line on a graph, representing measurements for a single
  test over time. A trace has a unique key, which is a combination of its
  properties (e.g., benchmark, test, subtest, bot, etc.).
- **Traceset**: The set of key-value pairs that uniquely identifies a trace.
- **X-axis**: Always represents commit position or timestamp.
- **Anomaly**: A statistically significant change in a trace, which could be a
  regression or an improvement.
- **Frame**: A chunk of trace data stored in the database.
- **Sheriff**: A person or tool responsible for monitoring a set of tests for
  regressions.
- **ChromePerf**: The legacy implementation of Perf.
- **Catapult**: The repository for the legacy Perf implementation.

See also
[Chromium Infra Glossary](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/infra/glossary.md).

## Production Instances

Below is the directory of active, public production Performance Dashboard
instances across various open-source projects. For the complete list, including
corp instances, refer to the configurations in the `perf/configs/` directory.

| Project / Dashboard   | Live URL                                                               | Description                                                                                    |
| :-------------------- | :--------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------- |
| **Skia Graphics**     | [skia-perf.luci.app](https://skia-perf.luci.app)                       | Public dashboard for the Skia 2D Graphics Library, tracking nanobenchmarks and task durations. |
| **Chromium (Public)** | [perf.luci.app](https://perf.luci.app)                                 | Public Chromium press benchmarks (Speedometer2, JetStream2, MotionMark).                       |
| **AndroidX**          | [androidx-perf.skia.org](https://androidx-perf.skia.org)               | Monitors performance and CQ tests for the Android Jetpack (AndroidX) support libraries.        |
| **ANGLE Graphics**    | [angle-perf.luci.app](https://angle-perf.luci.app)                     | Public GPU and graphics translation layer benchmarks (`angle_perftests`).                      |
| **Flutter Engine**    | [flutter-engine-perf.luci.app](https://flutter-engine-perf.luci.app)   | Low-level rendering speed and frame time metrics for the Flutter Engine.                       |
| **Flutter Framework** | [flutter-flutter-perf.luci.app](https://flutter-flutter-perf.luci.app) | Core framework-level UI performance, build speeds, and framework benchmarks.                   |
| **Fuchsia**           | [fuchsia-perf.luci.app](https://fuchsia-perf.luci.app)                 | Public telemetry, shell responsiveness, and system-level benchmarks for Fuchsia OS.            |
| **V8**                | [v8-perf.luci.app](https://v8-perf.luci.app)                           | Public JavaScript engine metrics, CQ dry-runs, and chromeperf benchmark runs.                  |
| **WebRTC**            | [webrtc-perf.luci.app](https://webrtc-perf.luci.app)                   | Audio/video connection performance and streaming metrics for WebRTC.                           |
| **Emscripten**        | [emscripten-perf.luci.app](https://emscripten-perf.luci.app)           | Performance and WebAssembly size metrics for Emscripten compiler releases.                     |
| **Germanium**         | [germanium-evals.luci.app](https://germanium-evals.luci.app)           | Public evaluation dashboard tracking Chromium main source performance estimates.               |

# Other Documentation

- [Skia Perf](https://skia.org/docs/dev/testing/skiaperf/): Documentation specifically focused on the Skia instance-specific features.
- [`ai_generated_doc.md`](./docs/ai_generated_doc.md): Overview of the system by Gemini.
- [`API.md`](./API.md): How to use the HTTP/JSON API for alerts.
- [`BACKUPS.md`](./BACKUPS.md): Instructions for backing up regression and alert data.
- [`CHECKLIST.md`](./CHECKLIST.md): A checklist for launching a new Perf instance.
- [`DESIGN.md`](./DESIGN.md): The design documentation for Perf.
- [`FORMAT.md`](./FORMAT.md): Details on the Perf JSON data format.
- [`PERFSERVER.md`](./PERFSERVER.md): Documentation for the `perfserver` command-line tool.
- [`PERFTOOL.md`](./PERFTOOL.md): Documentation for the `perf-tool` command-line tool.
- [`PROD.md`](./PROD.md): A manual for operating Perf in a production environment.
- [`Spanner.md`](./Spanner.md): Information on the Spanner integration and running the emulator.
- [`TRIAGE.md`](./TRIAGE.md): Design for the regression triage page.
