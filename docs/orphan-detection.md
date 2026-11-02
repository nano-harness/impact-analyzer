# Orphan Detection

`impact orphan` scans the whole repository by default and reports symbols that are not reachable from known entry points. It does not make hard assertions; each candidate is reported with `high`, `medium`, or `low` confidence and reasons.

## Entry points

Entry points come from framework contract extractors, scheduled-job extractors, `main`/`init`-style functions, optional test entries, and `.impact.yaml` `orphan.extra_entries`.

```yaml
orphan:
  extra_entries:
    - "internal/scheduler.RegisterJob"
    - pattern: ".*Cron$"
    - file_pattern: "**/jobs/**"
  heuristics:
    enable_filename: true
    enable_init: true
    enable_reflection: true
    enable_test_entry: false
  ignore:
    - "internal/legacy/**"
  confidence:
    new_code_weight: 3.0
    file_isolated_weight: 2.0
    has_old_version_weight: 2.0
    exported_offset: -1.5
```

## Usage

```bash
impact orphan
impact orphan --min-confidence high
impact orphan --diff HEAD~5 --format json
impact orphan --include-tests --format markdown
```

Python analysis is conservative because decorators, `getattr`, and dynamic imports may hide runtime entry points. Add dynamic entries to `orphan.extra_entries` when needed.
