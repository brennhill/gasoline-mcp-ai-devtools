---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Setting Up CRX Distribution on cookwithgasoline.com

## TL;DR

Host the CRX file and serve it as a direct download. Users drag-and-drop to install.

## Directory Structure

```
cookwithgasoline.com/
├── downloads/
│   ├── gasoline-extension-v5.8.2.crx     # Latest release
│   ├── gasoline-extension-v5.8.1.crx     # Archive
│   ├── gasoline-extension-v5.8.0.crx     # Archive
│   ├── latest.crx                         # Symlink to v5.8.2.crx (optional)
│   └── checksums.txt                      # SHA256 hashes (optional)
└── install/
    └── index.html                         # Installation guide (see below)
```

## Installation Landing Page

Create `install/index.html`:

```html
<!DOCTYPE html>
<html>
<head>
  <title>Install Gasoline Extension</title>
  <style>
    body { font-family: system-ui; max-width: 600px; margin: 40px auto; }
    .steps { list-style: none; counter-reset: step; }
    .steps li {
      margin: 20px 0;
      counter-increment: step;
      padding-left: 40px;
      position: relative;
    }
    .steps li:before {
      content: counter(step);
      position: absolute;
      left: 0;
      width: 30px;
      height: 30px;
      background: #333;
      color: white;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .download-btn {
      display: inline-block;
      padding: 12px 24px;
      background: #4285f4;
      color: white;
      text-decoration: none;
      border-radius: 4px;
      font-weight: 600;
      margin: 20px 0;
    }
    .note {
      background: #f0f0f0;
      padding: 12px;
      border-left: 4px solid #666;
      margin: 20px 0;
    }
  </style>
</head>
<body>
  <h1>Install Gasoline Extension</h1>
  <p>Follow these steps to install the Gasoline browser extension:</p>

  <a href="/downloads/latest.crx" class="download-btn">↓ Download Gasoline</a>

  <ol class="steps">
    <li>
      <strong>Open Chrome Extensions</strong>
      <p>Navigate to <code>chrome://extensions/</code> in your browser</p>
    </li>
    <li>
      <strong>Enable Developer Mode</strong>
      <p>Toggle "Developer mode" in the top-right corner</p>
    </li>
    <li>
      <strong>Drag & Drop</strong>
      <p>Drag the downloaded <code>.crx</code> file into the extensions page</p>
    </li>
    <li>
      <strong>Confirm Installation</strong>
      <p>Click "Add extension" in the confirmation dialog</p>
    </li>
  </ol>

  <div class="note">
    <strong>Note:</strong> You'll see a "Developer mode extensions" warning. This is normal for extensions installed outside the Chrome Web Store.
  </div>

  <hr>
  <h2>What's Gasoline?</h2>
  <p>Gasoline is a real-time browser telemetry tool that captures network requests, performance metrics, and user interactions. All data stays local on your machine.</p>

  <h2>Next Steps</h2>
  <ul>
    <li>Read the <a href="https://github.com/anthropics/gasoline/blob/main/README.md">documentation</a></li>
    <li>Join our <a href="https://discord.gg/example">community</a></li>
    <li>Report issues on <a href="https://github.com/anthropics/gasoline/issues">GitHub</a></li>
  </ul>

  <hr>
  <footer>
    <small>
      Gasoline v5.8.2 |
      <a href="/downloads/checksums.txt">Verify checksums</a> |
      <a href="https://github.com/anthropics/gasoline">View source</a>
    </small>
  </footer>
</body>
</html>
```

## Upload Process

### Automated (Recommended)

Add to your CI/CD pipeline:

```bash
#!/bin/bash
# deploy-extension.sh

VERSION=$(cat VERSION)
CRX_FILE="dist/gasoline-extension-v${VERSION}.crx"

# Build CRX
make extension-crx

# Upload to hosting
scp "$CRX_FILE" user@cookwithgasoline.com:/var/www/downloads/

# Update symlink
ssh user@cookwithgasoline.com \
  "cd /var/www/downloads && ln -sf gasoline-extension-v${VERSION}.crx latest.crx"

# Generate checksums
ssh user@cookwithgasoline.com \
  "cd /var/www/downloads && shasum -a 256 *.crx > checksums.txt"
```

### Manual

```bash
# 1. Build locally
make extension-crx

# 2. Upload to your server
scp dist/gasoline-extension-v5.8.2.crx \
  you@cookwithgasoline.com:/path/to/downloads/

# 3. SSH and update symlink
ssh you@cookwithgasoline.com
cd /path/to/downloads
ln -sf gasoline-extension-v5.8.2.crx latest.crx
shasum -a 256 *.crx > checksums.txt
```

## Server Configuration

### nginx

```nginx
# /etc/nginx/sites-available/gasoline
server {
    listen 443 ssl http2;
    server_name cookwithgasoline.com;

    location /downloads/ {
        # Allow CRX download
        types {
            application/x-chrome-extension crx;
        }

        # Cache control (users will re-download new versions)
        add_header Cache-Control "public, max-age=3600";

        # Enable directory listing (optional)
        autoindex on;
        autoindex_exact_size off;
        autoindex_localtime on;
    }

    location /install/ {
        types {
            text/html html htm;
        }
        try_files $uri $uri/ /install/index.html;
    }
}
```

### Apache

```apache
# /etc/apache2/sites-available/gasoline.conf
<VirtualHost *:443>
    ServerName cookwithgasoline.com
    DocumentRoot /var/www

    <Directory /var/www/downloads>
        AddType application/x-chrome-extension .crx
        Options +Indexes
        Allow from all
    </Directory>

    <Directory /var/www/install>
        DirectoryIndex index.html
        Allow from all
    </Directory>
</VirtualHost>
```

## MIME Types

Ensure your web server serves `.crx` files with the correct MIME type:

```
application/x-chrome-extension
```

This ensures:
- Chrome recognizes it as an installable extension
- Browsers don't try to display it as HTML

## Security

- ✅ HTTPS only (modern browsers require it)
- ✅ CRX files are cryptographically signed (Chrome verifies)
- ⚠️ Don't expose your private key (`~/.gasoline/extension-signing-key.pem`)
- ⚠️ Keep checksums for users to verify downloads

## Testing

Before going live:

```bash
# 1. Download the CRX
curl -o test.crx https://cookwithgasoline.com/downloads/gasoline-extension-v5.8.2.crx

# 2. Verify MIME type
file test.crx
# Should show: Google Chrome extension, version 3

# 3. Test installation in Chrome
# - Open chrome://extensions/
# - Enable Developer mode
# - Drag test.crx onto the page
# - Verify extension loads without errors
```

## User Support Links

Create a support page at `/support/install/`:

```
Q: Why do I need Developer mode?
A: Extensions distributed outside the Web Store require this for security transparency.

Q: Is this safe?
A: Yes. The .crx file is signed with our private key. Chrome verifies the signature before installation.

Q: Can I disable the Developer mode warning?
A: Not for extensions installed outside the Web Store. This is a Chrome security feature.

Q: How do I update?
A: Download the latest version and drag it onto chrome://extensions/ again.

Q: What if I want a Web Store version?
A: We're working on Web Store approval. Check back soon!
```

## Version Rollback

Keep old versions available for users who report issues:

```bash
# Keep at least 3 recent versions
/downloads/
  gasoline-extension-v5.8.2.crx  (current)
  gasoline-extension-v5.8.1.crx  (fallback)
  gasoline-extension-v5.8.0.crx  (archive)
  latest.crx → v5.8.2.crx
```

Users can downgrade by:
1. Visiting your downloads page
2. Selecting an older version
3. Re-installing

## Monitoring

Track downloads with web server logs:

```bash
# Analyze download patterns
tail -f /var/log/nginx/access.log | grep gasoline-extension
```

This helps you understand:
- How many users are installing
- Which versions are popular
- When to deprecate old versions
