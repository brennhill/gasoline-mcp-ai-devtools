---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Analytics Setup Guide for Gasoline MCP

This guide will help you set up analytics tracking for cookwithgasoline.com to measure marketing effectiveness and user behavior.

---

## Overview

We'll set up the following analytics tools:

1. **Google Analytics 4 (GA4)** - Primary analytics platform ✅ **Already configured (G-BDRZ2Z24M7)**
2. **GitHub Analytics** - Repository traffic and engagement
3. **NPM/PyPI Download Tracking** - Package installation metrics
4. **Social Media Analytics** - Twitter/X, LinkedIn, Reddit engagement
5. **Extension Install Tracking** - Chrome Web Store metrics

---

## 1. Google Analytics 4 Setup

### Step 1: Create GA4 Property

1. Go to [analytics.google.com](https://analytics.google.com)
2. Click "Start measuring" or "Admin" → "Create Account"
3. Account name: `Gasoline MCP`
4. Property name: `cookwithgasoline.com`
5. Reporting time zone: Your timezone
6. Currency: USD (or your preference)
7. Click "Create"

### Step 2: Get Measurement ID

1. After creating, you'll see your Measurement ID (format: `G-XXXXXXXXXX`)
2. Copy this ID for the next step

### Step 3: Add to Website

Add the GA4 tracking code to your website's `<head>` section:

```html
<!-- Google tag (gtag.js) -->
<script async src="https://www.googletagmanager.com/gtag/js?id=G-XXXXXXXXXX"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag('js', new Date());

  gtag('config', 'G-XXXXXXXXXX');
</script>
```

**For cookwithgasoline.com:**

If you're using a static site generator (Hugo, Jekyll, etc.), add this to your base template.

If you're using a CMS (WordPress, etc.), use the GA4 plugin or add to header.

### Step 4: Configure Custom Events

Track important user actions:

```javascript
// Track extension download
gtag('event', 'extension_download', {
  'event_category': 'downloads',
  'event_label': 'chrome_extension',
  'value': 1
});

// Track documentation page views
gtag('event', 'page_view', {
  'page_title': document.title,
  'page_location': window.location.href
});

// Track outbound link clicks (GitHub, Discord, etc.)
document.querySelectorAll('a[href^="http"]').forEach(link => {
  link.addEventListener('click', () => {
    gtag('event', 'outbound_click', {
      'event_category': 'outbound',
      'event_label': link.href
    });
  });
});
```

### Step 5: Set Up Goals/Conversions

In GA4 Admin → Events → Create conversions:

1. **Extension Download** - Track CRX file downloads
2. **Documentation View** - Track getting-started page views
3. **GitHub Visit** - Track clicks to GitHub repo
4. **Discord Join** - Track Discord link clicks

### Step 6: Enable Enhanced Measurement

In GA4 Admin → Data Streams → Your stream → Enhanced Measurement:

Enable:
- ✅ Page views
- ✅ Scroll tracking
- ✅ Outbound clicks
- ✅ Site search
- ✅ Video engagement
- ✅ File downloads

---

## 2. GitHub Analytics

### Repository Traffic

GitHub provides built-in analytics for repositories:

1. Go to your repo: https://github.com/brennhill/gasoline-mcp-ai-devtools
2. Click "Insights" tab
3. View:
   - **Traffic** - Views, clones, visitors
   - **Commits** - Contribution activity
   - **Code frequency** - Additions/deletions
   - **Punch card** - Commit times

### Track Stars Over Time

Use GitHub's API or third-party tools:

```bash
# Get star count
curl -s https://api.github.com/repos/brennhill/gasoline-mcp-ai-devtools | jq '.stargazers_count'
```

### Set Up GitHub Insights Dashboard

Consider using:
- [GitHub Stars History](https://stars.github.com/) - Track star growth
- [OSS Insight](https://ossinsight.io/) - Detailed repository analytics
- [LibHunt](https://www.libhunt.com/) - Compare with similar projects

---

## 3. NPM/PyPI Download Tracking

### NPM Downloads

```bash
# Get download stats for last 30 days
curl -s https://api.npmjs.org/downloads/point/last-month/gasoline-mcp

# Get download stats for specific version
curl -s https://api.npmjs.org/downloads/point/last-month/gasoline-mcp@6.0.0

# Get daily downloads for last week
curl -s https://api.npmjs.org/downloads/range/last-week/gasoline-mcp
```

### PyPI Downloads

```bash
# Get download stats
curl -s https://pypistats.org/api/packages/gasoline-mcp/recent

# Get overall stats
curl -s https://pypistats.org/api/packages/gasoline-mcp/overall
```

### Set Up Automated Tracking

Create a script to track downloads daily:

```bash
#!/bin/bash
# track-downloads.sh

DATE=$(date +%Y-%m-%d)
NPM_DOWNLOADS=$(curl -s https://api.npmjs.org/downloads/point/last-month/gasoline-mcp | jq '.downloads')
PYPI_DOWNLOADS=$(curl -s https://pypistats.org/api/packages/gasoline-mcp/recent | jq '.last_month')

echo "$DATE,$NPM_DOWNLOADS,$PYPI_DOWNLOADS" >> downloads.csv
```

Add to cron for daily tracking:
```bash
0 0 * * * /path/to/track-downloads.sh
```

---

## 4. Social Media Analytics

### Twitter/X Analytics

1. Go to [analytics.twitter.com](https://analytics.twitter.com)
2. View:
   - Impressions
   - Engagements (likes, retweets, replies, clicks)
   - Profile visits
   - Follower growth
   - Top tweets

### LinkedIn Analytics

1. Go to your LinkedIn Page
2. Click "Analytics" tab
3. View:
   - Visitor demographics
   - Post impressions
   - Engagement rates
   - Follower growth

### Reddit Analytics

Reddit doesn't provide built-in analytics, but you can:
- Track upvotes manually
- Use [Reddit Insight](https://redditinsight.com/) for post analysis
- Monitor comments and engagement

### Third-Party Tools

Consider using:
- [Buffer](https://buffer.com/) - Schedule and analyze posts
- [Hootsuite](https://hootsuite.com/) - Social media management
- [Sprout Social](https://sproutsocial.com/) - Advanced analytics

---

## 5. Chrome Web Store Analytics

### Extension Metrics

1. Go to [Chrome Web Store Developer Dashboard](https://chrome.google.com/webstore/devconsole)
2. Select your extension
3. View:
   - Install count
   - Active users
   - Ratings and reviews
   - Uninstall rate
   - Geographic distribution

### Track Installs

Set up a goal in GA4 for extension installs:

1. Create a custom event for extension downloads
2. Mark as conversion
3. Track over time

---

## 6. Key Metrics to Track

### Awareness Metrics
- **Website visitors** (GA4)
- **GitHub stars** (GitHub API)
- **Social media followers** (Platform analytics)
- **Extension installs** (Chrome Web Store)

### Acquisition Metrics
- **NPM/PyPI downloads** (Package registries)
- **Extension downloads** (GA4 events)
- **GitHub clones** (GitHub Insights)
- **Sign-ups** (Discord, newsletter)

### Engagement Metrics
- **Page views per session** (GA4)
- **Time on site** (GA4)
- **Bounce rate** (GA4)
- **Social media engagement** (Platform analytics)

### Retention Metrics
- **Returning visitors** (GA4)
- **Extension active users** (Chrome Web Store)
- **Repeat downloads** (NPM/PyPI)

---

## 7. Create a Dashboard

### Option 1: Google Looker Studio

1. Go to [lookerstudio.google.com](https://lookerstudio.google.com)
2. Create new report
3. Connect data sources:
   - Google Analytics 4
   - Google Sheets (for GitHub, NPM, PyPI data)
4. Build dashboard with key metrics

### Option 2: Google Sheets

Create a spreadsheet with tabs:
- **Overview** - Key metrics summary
- **Website** - GA4 data
- **GitHub** - Stars, clones, traffic
- **Downloads** - NPM, PyPI
- **Social** - Twitter, LinkedIn, Reddit

Use Google Apps Script to automate data updates:

```javascript
function updateGitHubStars() {
  var response = UrlFetchApp.fetch('https://api.github.com/repos/brennhill/gasoline-mcp-ai-devtools');
  var data = JSON.parse(response.getContentText());
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName('GitHub');
  sheet.getRange('B2').setValue(data.stargazers_count);
}
```

### Option 3: Third-Party Tools

Consider:
- [Mixpanel](https://mixpanel.com/) - Product analytics
- [Amplitude](https://amplitude.com/) - User behavior analytics
- [PostHog](https://posthog.com/) - Open-source analytics

---

## 8. Privacy Considerations

Gasoline MCP is privacy-focused, so ensure analytics respect user privacy:

1. **Anonymize IP addresses** in GA4:
   ```javascript
   gtag('config', 'G-XXXXXXXXXX', {
     'anonymize_ip': true
   });
   ```

2. **Disable ad personalization**:
   ```javascript
   gtag('config', 'G-XXXXXXXXXX', {
     'allow_google_signals': false
   });
   ```

3. **Add privacy policy** to your website explaining data collection

4. **Consider cookie consent** for GDPR compliance

---

## 9. Weekly Reporting Template

Create a weekly report to track progress:

```
Gasoline MCP Weekly Analytics Report
Week of: [DATE]

Website Metrics:
- Visitors: [NUMBER] ([% change])
- Page views: [NUMBER] ([% change])
- Avg session duration: [TIME] ([% change])
- Bounce rate: [%] ([% change])

GitHub Metrics:
- Stars: [NUMBER] ([% change])
- Clones: [NUMBER] ([% change])
- Visitors: [NUMBER] ([% change])

Downloads:
- NPM: [NUMBER] ([% change])
- PyPI: [NUMBER] ([% change])
- Extension: [NUMBER] ([% change])

Social Media:
- Twitter followers: [NUMBER] ([% change])
- Twitter impressions: [NUMBER] ([% change])
- LinkedIn followers: [NUMBER] ([% change])

Top Pages:
1. [PAGE] - [VIEWS]
2. [PAGE] - [VIEWS]
3. [PAGE] - [VIEWS]

Top Referrers:
1. [SOURCE] - [VISITORS]
2. [SOURCE] - [VISITORS]
3. [SOURCE] - [VISITORS]

Goals/Conversions:
- Extension downloads: [NUMBER]
- GitHub visits: [NUMBER]
- Discord joins: [NUMBER]

Notes:
- [Any notable events or changes]
```

---

## 10. Next Steps

### This Week:
- [x] Set up GA4 property ✅ (Already configured: G-BDRZ2Z24M7)
- [x] Add tracking code to website ✅ (Already in Jekyll config)
- [ ] Configure custom events
- [ ] Set up conversion goals
- [ ] Verify enhanced measurement is enabled

### Next Week:
- [ ] Set up GitHub tracking
- [ ] Configure NPM/PyPI tracking
- [ ] Set up social media analytics
- [ ] Create Looker Studio dashboard

### Ongoing:
- [ ] Review analytics weekly
- [ ] Adjust strategy based on data
- [ ] Track key metrics
- [ ] Report on progress

---

## Resources

- [GA4 Setup Guide](https://support.google.com/analytics/answer/9304153)
- [GitHub REST API](https://docs.github.com/en/rest)
- [NPM API](https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md)
- [PyPI Stats API](https://pypistats.org/api/)
- [Chrome Web Store Developer Dashboard](https://chrome.google.com/webstore/devconsole)

---

## Questions?

If you need help setting up analytics, join our Discord community or open an issue on GitHub.
