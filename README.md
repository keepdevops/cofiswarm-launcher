# cofiswarm-launcher

Cofiswarm component: `launcher`.

- Layout: [REPO-STANDARD-LAYOUT](https://github.com/keepdevops/cofiswarmdev/blob/main/docs/REPO-STANDARD-LAYOUT.md)
- Migration: [MIGRATION-SPRINTS](https://github.com/keepdevops/cofiswarmdev/blob/main/docs/MIGRATION-SPRINTS.md)

## FHS paths

| Path | Purpose |
|------|---------|
| `/etc/cofiswarm/launcher/` | config |
| `/var/lib/cofiswarm/launcher/` | state |
| `/var/log/cofiswarm/launcher/` | logs |

## Test

```bash
./test/scripts/assert-layout.sh launcher
```
