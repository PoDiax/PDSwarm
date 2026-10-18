# PDSwarm {VERSION}

> **Highlights:** Summary of this release.

---

### What's New
- **Feature Name**: Brief description.

### Improvements & Tweaks
- **Component / Area**: What was changed.

### Bug Fixes
- **Issue**: What was fixed.

---

### Updating to this version

**Installed as a Linux Service (systemd):**
```bash
sudo pdswarm-update
```
*(Or wait for your daily auto-update timer at 04:00 AM)*

**Docker Compose:**
```bash
docker compose pull && docker compose up -d
```

**Docker Run:**
```bash
docker pull ghcr.io/podiax/pdswarm:latest
docker stop pdswarm && docker rm pdswarm
# Run your usual docker run command
```

---

**Full Changelog**: https://github.com/podiax/pdswarm/compare/{PREVIOUS_TAG}...{VERSION}
