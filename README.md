# GitLab Pages Exporter (gpe)

Exporter gathering periodic statistics about GitLab pages via GitLab's API.

## Why This Exporter Exists

There is no builtin way in GitLab to find information about what projects
expose pages.

## How it Works

The exporter periodically scrapes the GitLab API to get information about pages 
deployments. Since there is no built-in way in GitLab to get all projects with 
pages deployments the exporter checks if a project has CI/CD enabled and if so
if there is a job named `pages` which was successfully run. If those conditions
are met, the exporter assumes pages are deployed.

[GitLab Pages Doc](https://docs.gitlab.com/ee/user/project/pages/#how-it-works)

**Note**: Scraping may issue a lot of requests depending on how large your 
instance is and you might need to check your rate-limiting settings.

## Exposed Metrics

| Metric Name                               | Description                       |
| ------------------------------------------|-----------------------------------|
| `gpe_project_pages_enabled`               | If GitLab pages are enabled       |
| `gpe_custom_Domains_verified`             | If a custom Domain is verified    |
| `gpe_projects_checked_total`              | Number of projects processed      |
| `gpe_check_running`                       | If the check is currently running |
| `gpe_last_check_duration_seconds`         | How long the last check took      |
| `gpe_last_check_run_finished_seconds`     | When the last check happened      |
| `gpe_next_check_run_scheduled_seconds`    | When the next check will happen   |

## Configuration

Currently configuration can only be achieved with environment variables, it is
recommended to use the docker image to run the exporter.

| Variable Name                  | Description                                                                                                                                                  | Default       |
| -------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| `GPE_GITLAB_ADMIN_READ_TOKEN`  | A read-only API token with admin permissions (to be able to fetch all projects).                                                                             | ``            |
| `GPE_GITLAB_API_URL`           | The base URL to your GitLab instance.                                                                                                                        | ``            |
| `GPE_CRON_SCHEDULE`            | Schedule for tests in cron format (seconds, minutes, hours, day of month, month, day of week).                                                               | `0 0 2 * * *` |
| `GPE_SET_ALL_PROJECT_METRICS`  | If all projects should be exposed as metric, by default only project with pages deployed are exposed. Only set to true if really needed as this grows quick! | `false`       |

## Author(s)

Originally developed at Bern University of Applied Sciences (TI): [BFH](https://www.bfh.ch/ti/en/)