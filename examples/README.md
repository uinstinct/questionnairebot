# Examples

Reference YAML files showing the on-disk shape the bot reads and writes. These
are **not loaded at runtime** — `DATA_DIR` (default `./data`) is the live
directory. Copy a subdirectory from here into `data/` to try the bot end-to-end:

```bash
cp -r examples/daily-standup data/daily-standup
docker compose up -d
```

## Files

- `daily-standup/questionnaire.yaml` — questionnaire definition (cron, timezone,
  questions). Read-only at runtime.
- `daily-standup/answers.yaml` — completed/skipped session log, newest entry
  first. The bot prepends to this file.

A `session.yaml` (in-progress state) is created automatically when a session
starts and deleted when it completes; no example is shipped because nothing on
disk should ever be authored by hand. See `original-prd.md` §4.3 for its shape.
