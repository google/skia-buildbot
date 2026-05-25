# Multigraph User Guide

> **Bug Filing & Feedback**: Found an issue with the Perf Multigraph Page,
> or have a feature request? File a ticket in the
> [Perf 2.0 Buganizer Component](https://b.corp.google.com/issues/new?component=1989668).

This guide explains how to effectively search and filter multiple performance graphs within
the Browser Performance Infrastructure. While the **Explore** (`/e`) view is designed for
deep-diving into a single, comprehensive graph, and **Multigraph** (`/m`) is optimized
for viewing and synchronizing multiple metrics simultaneously, the **Report Page** (`/u`) is the
dedicated workspace for triaging anomalies and regressions. (See the
[Report Page Triage & Analysis Guide](./report-page-guide.md) for full details).

Note: All examples and links in this guide use the Chrome Perf instance
([https://chrome-perf.corp.goog/m/](https://chrome-perf.corp.goog/m/)), but the
concepts and instructions apply to all **Perf 2.0** dashboard instances.

# Table of Contents

- [Accessing Multigraph](#accessing-multigraph)
- [Using the Test Picker](#using-the-test-picker)
- [Sync Mode (Default)](#sync-mode-default)
- [Independent Graph Mode](#independent-graph-mode)

## Accessing Multigraph

You can access the Multigraph view in two ways:

- **Direct Access**: Navigate directly to the `/m` path (e.g., [https://chrome-perf.corp.goog/m/](https://chrome-perf.corp.goog/m/)).
- **Sidebar**: Click the **Multigraph** button on the left navigation sidebar.

Note: This is the default view for most dashboard instances.

## Using the Test Picker

The **Test Picker** is the primary tool for building queries to find specific metrics. It operates on a dynamic key-value hierarchy:

![Test Picker Selections](./images/multigraph_guide/test-picker-selections.webp 'Test Picker Selections')

- **Key-Value Selection**: The data is categorized by keys (e.g., `benchmark`, `bot`, `test`). When you select a key, you are prompted to select one or more matching values.
- **Configurable Hierarchy**: The order of keys (e.g., `benchmark`, `bot`, `test`) is determined by the `include_params` setting in the instance configuration. This allows the search hierarchy to be customized based on how the data is indexed (see [example configuration](https://skia.googlesource.com/buildbot/+/8828d33644b3127fe4aac7738a47ff5e6c77fa68/perf/configs/spanner/chrome-internal.json#121)). Keys not specified in the hierarchy are not set by default, but can be assigned default values via `default_param_selections` (see [example](https://skia.googlesource.com/buildbot/+/8828d33644b3127fe4aac7738a47ff5e6c77fa68/perf/configs/spanner/chrome-internal.json#130)).
- **Dynamic Filtering**: Your selections actively filter the dataset. Once you select a value for one key, the options for all subsequent keys are narrowed down to only show values that match your current query.
- **Traces Counter**: As you build your query, watch the **Traces** counter. This number represents the total count of individual time-series lines (traces) that match your current filters.
- **Plotting**: You can click the **Plot** button to render the graph once your selections are well-defined. Narrow down your choices until the **Traces** counter shows a manageable number to avoid overloading the dashboard.

## Sync Mode (Default)

Multigraph operates in Sync Mode by default. In this mode, the graphs are **tightly coupled** to the Test Picker. This is optimized for **rapid exploration** and **comparative analysis**: as you adjust filters in the Test Picker, the graphs update instantly, allowing you to quickly correlate performance changes across different metrics or bot configurations without manual effort.

To plot the initial graph, select the desired parameters and click the **Plot** button in the Test Picker. This will plot a single graph with the selected query.

![Click plot button](./images/multigraph_guide/click-plot-button-test-picker.png 'Click plot button')

![First trace plotted](./images/multigraph_guide/first-trace-after-plot.png 'First trace plotted')

### Adding a Trace

You can add a trace by selecting a value in the Test Picker that narrows your query down to a unique trace. It will automatically be plotted on the chart.

Selecting an additional value in the same field...

![Select second trace](./images/multigraph_guide/select-second-trace-field.png 'Select second trace')

...will instantly plot the new trace on the same chart alongside the first one.

![Second trace plotted](./images/multigraph_guide/second-trace-after-select.png 'Second trace plotted')

### Removing a Trace

You can remove a trace by de-selecting its value in the Test Picker. This immediately removes the corresponding trace from the chart.

![Removing a trace](./images/multigraph_guide/remove-trace.png 'Removing a trace')

### Selecting All Options

When you want to quickly add all available values for a specific field to your chart, you can check the "All" box next to that field in the Test Picker.

![Select all options](./images/multigraph_guide/click-all.png 'Select all options')

This will simultaneously plot all matching traces on the chart.

![All traces plotted](./images/multigraph_guide/all-traces.png 'All traces plotted')

### Splitting Graphs

You can create multiple synchronized graphs based on a specific parameter by toggling the "Split" button next to any key in the Test Picker.

![Click split button](./images/multigraph_guide/select-split.png 'Click split button')

For example, splitting by `bot` will generate a separate graph for each bot matching your query.

![Split graphs](./images/multigraph_guide/split-graphs.png 'Split graphs')

The separate graphs will remain synchronized as you zoom or pan across the timeline.

![Synchronized graphs](./images/multigraph_guide/sync-graphs-plot-summary.png 'Synchronized graphs')

## Independent Graph Mode

**Independent Graph Mode** lets you build custom dashboards with completely unrelated metrics. It is currently enabled by adding `?manual_plot_mode=true` to the URL, but can also be [set as the default via a configuration change](https://skia.googlesource.com/buildbot/+/8828d33644b3127fe4aac7738a47ff5e6c77fa68/perf/configs/spanner/fuchsia-internal.json#87). Note that in this mode, selections in the Test Picker will not affect the displayed graph unless the **Plot** button is clicked, which will render a new graph.

### Plotting Graphs

To plot a new graph, select the desired parameters and click the **Plot** button in the Test Picker.

![Click plot button](./images/multigraph_guide/click-plot-button-test-picker.png 'Click plot button')

This will generate a new chart at the top of the page with the traces matching your current query.

![First trace plotted](./images/multigraph_guide/first-trace-after-plot.png 'First trace plotted')

Each **Plot** click generates a new graph. For example, selecting a different query and clicking **Plot** again...

![Plot second trace](./images/multigraph_guide/plot-second-trace-independent.png 'Plot second trace')

...will render a completely separate, independent graph above the first one.

![Two independent graphs](./images/multigraph_guide/two-graphs-independent.png 'Two independent graphs')

### Populating the Test Picker

If you have a trace that you want to use as a starting point for a new search, you can click the **Query Highlighted** button on any chart. This will backfill the Test Picker with all the keys and values that define that specific trace.

![Populating the test picker button](./images/multigraph_guide/populate-test-picker-button.png 'Populating the test picker button')

### Removing a Graph

To remove an independent graph, click the **X** button located in the top-right corner of the chart container.

![Removing a graph](./images/multigraph_guide/remove-graph-button.png 'Removing a graph')
