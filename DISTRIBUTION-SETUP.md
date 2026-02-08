# Gasoline CRX Distribution Setup — Complete

## ✅ What's Been Done

### 1. Generated Signing Key
- **Location:** `~/.gasoline/extension-signing-key.pem`
- **Type:** RSA-2048 with SHA-256 signing
- **Status:** Secure, backed up separately (not in git)

### 2. Built CRX Script
- **Location:** `scripts/build-crx.js`
- **Features:**
  - Generates CRX v3 (Chrome Extension v3 format)
  - Auto-computes extension ID from public key
  - Signs with SHA-256
  - ~5 second build time
- **Status:** Tested and working

### 3. Makefile Target
```bash
make extension-crx   # Builds signed CRX file
```

### 4. Documentation
- **[docs/distribution/extension-crx.md](docs/distribution/extension-crx.md)** — Technical reference, key mgmt, release workflow
- **[docs/distribution/website-setup.md](docs/distribution/website-setup.md)** — Server setup, installation page template, CI/CD integration

---

## 📦 Current Extension ID

```
behrmkvjipzkr7hu6mwmbt5vpdgcdyvk
```

This ID is **locked to your private key**. Same key = same ID forever.

---

## 🚀 Quick Start: Release a Version

### 1. Build the CRX
```bash
make extension-crx
```

Output:
```
✨ CRX file created: dist/gasoline-extension-v5.8.2.crx
📊 File size: 345.9 KB
📦 Extension ID: behrmkvjipzkr7hu6mwmbt5vpdgcdyvk
```

### 2. Upload to cookwithgasoline.com
```bash
scp dist/gasoline-extension-v5.8.2.crx \
  you@cookwithgasoline.com:/var/www/downloads/

ssh you@cookwithgasoline.com
cd /var/www/downloads
ln -sf gasoline-extension-v5.8.2.crx latest.crx
shasum -a 256 *.crx > checksums.txt
```

### 3. Users Install
- Visit: https://cookwithgasoline.com/install/
- Download: gasoline-extension-v5.8.2.crx
- Install: Drag to `chrome://extensions/`

---

## 📋 File Manifest

### New Files
- `scripts/build-crx.js` — CRX builder (Node.js, ES modules)
- `docs/distribution/extension-crx.md` — CRX technical docs
- `docs/distribution/website-setup.md` — Website hosting guide
- `DISTRIBUTION-SETUP.md` — This file

### Modified Files
- `Makefile` — Added `extension-crx` target

### Generated (not in git)
- `~/.gasoline/extension-signing-key.pem` — Your private signing key ⚠️ **KEEP SECURE**
- `dist/gasoline-extension-v*.crx` — Build output

---

## 🔐 Key Security Points

✅ **Safe:**
- Private key never embedded in CRX
- CRX signed and Chrome-verified
- All data local (no external transmission)

⚠️ **Important:**
- Keep `~/.gasoline/extension-signing-key.pem` secure and backed up
- Never commit the key to git
- Losing the key = can't update existing installations
- This is separate from your SSH key (not reused)

---

## 📊 Side-by-Side: Distribution Options

| Aspect | CRX (Now) | Web Store (Later) |
|--------|-----------|------------------|
| **Time to Users** | Immediate | 1-2 weeks (Google review) |
| **Installation** | Drag-drop (easy) | 1-click (easiest) |
| **Updates** | Manual | Automatic |
| **Control** | 100% yours | Google's systems |
| **Extension ID** | Locked to your key | Google assigns |
| **User Warning** | Developer mode warning | None |

---

## 🎯 Recommended Workflow

### Now (While Waiting for Web Store)
```
1. make extension-crx
2. Upload to cookwithgasoline.com/downloads/
3. Users download and drag-drop to install
```

### After Web Store Approval
```
Option A (Keep Both):
- CRX version stays available on your website
- Web Store version for mainstream users
- Users effectively have 2 separate extension instances

Option B (Migrate):
- Download the signed CRX from Web Store
- Host that instead of self-signed
- Users upgrade to Web Store version (same extension ID)
- Deprecate self-signed version
```

---

## 🔄 Release Checklist

- [ ] Run `make compile-ts` — Rebuild extension from TypeScript
- [ ] Run `make test-js` — Verify extension tests pass
- [ ] Run `make extension-crx` — Generate signed CRX
- [ ] Verify `dist/gasoline-extension-v[VERSION].crx` exists
- [ ] Upload to `cookwithgasoline.com/downloads/`
- [ ] Test download works
- [ ] Test installation on fresh Chrome profile
- [ ] Verify extension ID matches: `behrmkvjipzkr7hu6mwmbt5vpdgcdyvk`
- [ ] Update symlink: `latest.crx → gasoline-extension-v[VERSION].crx`
- [ ] Generate checksums: `shasum -a 256 *.crx > checksums.txt`
- [ ] Git commit & push (don't commit .crx files)

---

## 🆘 Troubleshooting

| Problem | Solution |
|---------|----------|
| "Private key not found" | Run: `mkdir -p ~/.gasoline && openssl genrsa 2048 > ~/.gasoline/extension-signing-key.pem` |
| Extension ID mismatch | Different key = different ID. Check you're using the right key file. |
| CRX won't install | Run `make compile-ts` first, then `make extension-crx` |
| "Not a Chrome extension" error | Verify MIME type: `file dist/*.crx` should show "Google Chrome extension, version 3" |
| Users need update | No auto-update yet. They manually download and re-install. |

---

## 📈 Next Steps

1. ✅ **Immediate:** Start distributing CRX from cookwithgasoline.com
2. **Soon:** Set up installation landing page (template in website-setup.md)
3. **Later:** Implement auto-update manifest (see extension-crx.md for details)
4. **Future:** Migrate to Web Store after approval

---

## 📚 Full Documentation

- **Technical Details:** [docs/distribution/extension-crx.md](docs/distribution/extension-crx.md)
- **Website Setup:** [docs/distribution/website-setup.md](docs/distribution/website-setup.md)
- **Build Script:** [scripts/build-crx.js](scripts/build-crx.js)
- **Makefile:** Search for `extension-crx` target

---

## Questions?

- **How do users get updates?** Currently they download the new .crx and re-install. Auto-update manifests can be set up later.
- **What about the Web Store?** Keep this setup alongside Web Store version—they're separate extension instances.
- **Can I change the extension ID?** Only by generating a new key. The ID is derived from your public key hash.
- **Is this secure?** Yes. Chrome verifies the CRX signature before installation.

---

**Generated:** 2026-02-08
**Version:** Gasoline v5.8.2
**Extension ID:** behrmkvjipzkr7hu6mwmbt5vpdgcdyvk
