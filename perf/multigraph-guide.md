# Multigraph User Guide

[go/browser-perf-multigraph](http://go/browser-perf-multigraph)

This guide explains how to effectively search and filter multiple performance graphs within the Browser Performance Infrastructure. While the **Explore** (`/e`) view is designed for deep-diving into a single, comprehensive graph, **Multigraph** (`/m`) is optimized for viewing and synchronizing multiple metrics simultaneously.

Note: All examples and links in this guide use the Chrome Perf instance ([https://chrome-perf.corp.goog/m/](https://chrome-perf.corp.goog/m/)), but the concepts and instructions apply to all Performance Dashboard instances.

# Table of Contents

- [Accessing Multigraph](#accessing-multigraph)
- [Using the Test Picker](#using-the-test-picker)
<!-- TODO(eduardoyap): Add sections for Sync Mode and Manual Plot Mode (b/491576811) -->

## Accessing Multigraph

You can access the Multigraph view in two ways:

- **Direct Access**: Navigate directly to the `/m` path (e.g., [https://chrome-perf.corp.goog/m/](https://chrome-perf.corp.goog/m/)).
- **Sidebar**: Click the **Multigraph** button on the left navigation sidebar.

Note: This is the default view for most dashboard instances.

![Multigraph Sidebar Button](./images/multigraph-sidebar.png 'Multigraph Sidebar')

## Using the Test Picker

The **Test Picker** is the primary tool for building queries to find specific metrics. It operates on a dynamic key-value hierarchy:

![Test Picker Interface](./images/test-picker.png 'Test Picker Interface')

- **Key-Value Selection**: The data is categorized by keys (e.g., `benchmark`, `bot`, `test`). When you select a key, you are prompted to select one or more matching values.
- **Configurable Hierarchy**: The order of keys (e.g., `benchmark`, `bot`, `test`) is determined by the `include_params` setting in the instance configuration. This allows the search hierarchy to be customized based on how the data is indexed (see [example configuration](https://source.corp.google.com/h/skia/buildbot/+/main:perf/configs/spanner/chrome-internal.json;l=121)). Keys not specified in the hierarchy are not set by default, but can be assigned default values via `default_param_selections` (see [example](https://source.corp.google.com/h/skia/buildbot/+/main:perf/configs/spanner/chrome-internal.json;l=129)).
- **Dynamic Filtering**: Your selections actively filter the dataset. Once you select a value for one key, the options for all subsequent keys are narrowed down to only show values that match your current query.
- **Traces Counter**: As you build your query, watch the **Traces** counter. This number represents the total count of individual time-series lines (traces) that match your current filters.
- **Plotting**: You can click the **Plot** button to render the graph once your selections are well-defined. Narrow down your choices until the **Traces** counter shows a manageable number to avoid overloading the dashboard.
