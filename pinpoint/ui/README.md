# Pinpoint UI Directory Overview

This directory contains the frontend code for the Pinpoint application, built using
LitElement for web components and TypeScript. The UI is structured to promote modularity and
reusability, with clear separation of concerns between pages, reusable components (modules), and API
services.

## How the UI is Constructed

The Pinpoint UI is built by combining these elements:

**Pages as Entry Points**: The `pages` directory contains the root HTML files (e.g., `landing-page.
html`, `results-page.html`). These HTML files are minimal, primarily serving as containers for the
main LitElement components that define the page's layout and functionality. For example,
`landing-page.html` simply includes `<pinpoint-landing-page-sk></pinpoint-landing-page-sk>`.

**Modules as Building Blocks**: The `modules` directory contains the actual LitElement
web components. These components encapsulate specific UI functionalities (e.g., displaying
a table of jobs, a form for creating a new job, or a detailed view of a job's runs). Pages
import and use these modules to construct their views. For instance, `pinpoint-landing-page-sk`
imports and uses `jobs-table-sk` to display the list of jobs.

**Services for Backend Communication**: The `services/api.ts` file acts as the central point
for all API interactions. It exports asynchronous functions (e.g., `listJobs`, `getJob`
`schedulePairwise`) that handle fetching and sending data to the Pinpoint backend. In addition,
it also defines TypeScript interfaces (e.g., `Job`, `JobSchema`, `SchedulePairwiseRequest`) that
directly correspond to the Go structs used by the backend API. This ensures that the data
consumed and produced by the frontend matches the backend's expectations.

## Module Descriptions

Here's a short description of each module in the `pinpoint/ui/modules` directory:

- **`commit-run-overview-sk`**: Displays a summary of a commit's build and test runs,
  including build status, Swarming task links, and a visual representation of individual test run
  outcomes. It also provides a dialog for detailed run information.
- **`job-overview-sk`**: Presents a modal dialog that shows the detailed arguments and
  parameters used to create a specific Pinpoint job.
- **`jobs-table-sk`**: Renders a sortable table of Pinpoint jobs, allowing users to view job
  details, status, and initiate actions like cancellation for running jobs.
- **`pinpoint-landing-page-sk`**: The main component for the Pinpoint landing page, responsible
  for fetching and displaying a paginated and filterable list of jobs using the `jobs-table-sk`
  component. It also handles job cancellation.
- **`pinpoint-new-job-sk`**: Provides a modal dialog for users to create and schedule new Pinpoint
  jobs (pairwise or bisection). It includes forms for specifying commits, benchmarks, bots, and
  other job parameters.
- **`pinpoint-results-page-sk`**: Displays the detailed results for a specific Pinpoint job,
  including an overview of the base and experimental commit runs, and the Wilcoxon statistical
  analysis results if applicable.
- **`pinpoint-scaffold-sk`**: Serves as the main application shell, providing a consistent header
  with search functionality, filter options, and a button to open the "Create a new job" modal.
- **`wilcoxon-results-sk`**: Displays the statistical Wilcoxon test results for pairwise Pinpoint
  jobs, showing metrics like p-value, confidence intervals, and percentage change, along with an
  indication of significance and improvement/regression.
