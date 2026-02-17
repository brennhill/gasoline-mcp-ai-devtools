---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Discord Server Setup Guide for Gasoline MCP

This guide will help you set up a Discord community for Gasoline MCP users and contributors.

---

## Overview

A Discord server will provide:
- Real-time support and help
- Community discussions and feedback
- Feature request and bug reporting
- Contributor coordination
- Weekly events and announcements

---

## Step 1: Create the Discord Server

### 1.1 Create Server

1. Go to [discord.com](https://discord.com) and log in
2. Click the "+" icon on the left sidebar
3. Select "Create My Own"
4. Choose "For me and my friends" or "For a club or community"
5. Server name: `Gasoline MCP`
6. Upload server icon (use Gasoline logo)
7. Click "Create"

### 1.2 Server Settings

Go to Server Settings → Overview:

- **Server Name:** Gasoline MCP
- **Server Icon:** Upload Gasoline logo
- **Server Banner:** Upload banner image
- **Server Region:** Choose closest to your audience
- **Verification Level:** Medium (recommended for security)
- **Explicit Content Filter:** All members
- **Two-Factor Authentication:** Require for moderators

---

## Step 2: Create Channels

### 2.1 Information Channels

Create these channels in the "Information" category:

| Channel | Type | Purpose |
|---------|------|---------|
| `#welcome` | Text | Welcome message and getting started |
| `#announcements` | Text (Announcement) | Official updates and releases |
| `#rules` | Text | Community guidelines |
| `#resources` | Text | Links to docs, GitHub, etc. |
| `#faq` | Text | Frequently asked questions |

### 2.2 Support Channels

Create these channels in the "Support" category:

| Channel | Type | Purpose |
|---------|------|---------|
| `#help` | Text | General help and support |
| `#installation` | Text | Installation issues |
| `#bug-reports` | Text | Bug reports and issues |
| `#feature-requests` | Text | Feature suggestions |

### 2.3 Discussion Channels

Create these channels in the "Discussion" category:

| Channel | Type | Purpose |
|---------|------|---------|
| `#general` | Text | General discussion |
| `#showcase` | Text | Share projects and use cases |
| `#ai-tools` | Text | Discuss AI coding tools (Claude, Cursor, etc.) |
| `#web-dev` | Text | Web development discussion |
| `#off-topic` | Text | Casual conversation |

### 2.4 Development Channels

Create these channels in the "Development" category:

| Channel | Type | Purpose |
|---------|------|---------|
| `#development` | Text | Development discussion |
| `#contributing` | Text | Contribution guidelines |
| `#pull-requests` | Text | PR discussion |
| `#code-review` | Text | Code review requests |

### 2.5 Voice Channels

Create these voice channels:

| Channel | Purpose |
|---------|---------|
| 🎙️ General | General voice chat |
| 🎙️ Office Hours | Weekly office hours |
| 🎙️ Pair Programming | Collaborative coding |
| 🎙️ Events | Community events |

---

## Step 3: Configure Channel Permissions

### 3.1 Default Role (@everyone)

For most channels, set @everyone permissions:

**Read Messages:** ✅  
**Send Messages:** ✅  
**Add Reactions:** ✅  
**Embed Links:** ✅  
**Attach Files:** ✅  
**Read Message History:** ✅  

**Exceptions:**
- `#announcements`: Send Messages ❌ (only moderators)
- `#rules`: Send Messages ❌ (read-only)
- `#resources`: Send Messages ❌ (read-only)

### 3.2 Moderator Role

Create a "Moderator" role with additional permissions:

- Manage Messages
- Manage Channels
- Mute Members
- Deafen Members
- Move Members
- Priority Speaker

### 3.3 Contributor Role

Create a "Contributor" role for users who have contributed:

- Custom role color
- Display role separately from online members
- Access to `#development` channel

---

## Step 4: Set Up Bots

### 4.1 MEE6 (Moderation & Welcome)

1. Go to [mee6.xyz](https://mee6.xyz)
2. Add to your server
3. Configure:
   - Welcome messages in `#welcome`
   - Auto-moderation rules
   - Level system (optional)
   - Custom commands

### 4.2 Carl-bot (Reaction Roles)

1. Go to [carl.gg](https://carl.gg)
2. Add to your server
3. Set up reaction roles:
   - 📢 Announcements role
   - 💻 Developer role
   - 🐛 Bug hunter role
   - ✨ Contributor role

### 4.3 Statbot (Server Statistics)

1. Go to [statbot.net](https://statbot.net)
2. Add to your server
3. Configure statistics channels:
   - Member count
   - Online members
   - Message count

### 4.4 GitHub Bot (Optional)

Consider a bot to sync with GitHub:
- [GitHub Discord Bot](https://github.com/Discord4J/Discord4J)
- [Sentry](https://sentry.io/) for error tracking

---

## Step 5: Create Channel Content

### 5.1 #welcome Channel

```
👋 Welcome to Gasoline MCP!

Gasoline MCP is a browser observability tool for AI coding agents. 
It gives your AI assistant real-time visibility into browser activity.

🚀 Get Started:
• Documentation: https://cookwithgasoline.com
• Quick Start: https://cookwithgasoline.com/getting-started/
• GitHub: https://github.com/brennhill/gasoline-mcp-ai-devtools

❓ Need Help?
• Check #faq for common questions
• Ask in #help for support
• Report bugs in #bug-reports

💬 Join the Conversation:
• Introduce yourself in #general
• Share your projects in #showcase
• Join our weekly office hours in 🎙️ Office Hours

📢 Stay Updated:
• Enable the 📢 Announcements role for updates
• Follow @gasolinedev on Twitter
• Star us on GitHub!

🎉 Welcome to the community!
```

### 5.2 #announcements Channel

```
📢 **Announcements**

This channel is for official Gasoline MCP updates, releases, and news.

Enable the 📢 Announcements role to get notified of new announcements!
```

### 5.3 #rules Channel

```
📜 **Community Guidelines**

1. Be respectful and kind to all members
2. No harassment, hate speech, or discrimination
3. No spam or self-promotion (unless relevant)
4. Keep discussions on-topic
5. Respect privacy - don't share personal info
6. Follow Discord's Terms of Service
7. Ask for help before DMing moderators

⚠️ Violations may result in warnings, mutes, or bans.

Report issues to moderators via DM or in #help.
```

### 5.4 #resources Channel

```
📚 **Resources**

🔗 Official Links:
• Website: https://cookwithgasoline.com
• Documentation: https://cookwithgasoline.com/docs/
• GitHub: https://github.com/brennhill/gasoline-mcp-ai-devtools
• Chrome Extension: https://cookwithgasoline.com/downloads/

📖 Documentation:
• Getting Started Guide
• MCP Integration
• Feature Documentation
• API Reference

🛠️ AI Tools:
• Claude Code: https://claude.ai/code
• Cursor: https://cursor.sh
• Windsurf: https://windsurf.ai
• Zed: https://zed.dev

🤝 Community:
• Twitter: https://twitter.com/gasolinedev
• Discord Invite: [Your invite link]
• GitHub Discussions: [Link]
```

### 5.5 #faq Channel

```
❓ **Frequently Asked Questions**

Q: How do I install Gasoline MCP?
A: See the Quick Start guide: https://cookwithgasoline.com/getting-started/

Q: Which AI tools does Gasoline work with?
A: Claude Code, Cursor, Windsurf, Zed, Claude Desktop, VS Code + Continue, and any MCP-compatible tool.

Q: Is Gasoline free?
A: Yes! Gasoline is open source and free to use.

Q: Does Gasoline send my data to the cloud?
A: No. All data stays on your machine. No cloud, no telemetry.

Q: How do I report a bug?
A: Use the #bug-reports channel or open an issue on GitHub.

Q: How can I contribute?
A: Check the #contributing channel or see CONTRIBUTING.md on GitHub.

Q: Where can I get help?
A: Ask in #help or join our weekly office hours.

Still have questions? Ask in #help!
```

---

## Step 6: Set Up Roles

### 6.1 Create Roles

Go to Server Settings → Roles → Create Role:

| Role | Color | Permissions | Purpose |
|------|-------|-------------|---------|
| @everyone | Gray | Default | All members |
| 📢 Announcements | Blue | None | Receive announcements |
| 💻 Developer | Green | None | Identify as developer |
| 🐛 Bug Hunter | Orange | None | Bug reporters |
| ✨ Contributor | Purple | +Dev channel | Contributors |
| 🔧 Moderator | Red | Full moderation | Server moderators |
| 👑 Admin | Gold | Administrator | Server owner |

### 6.2 Reaction Roles

Set up in `#welcome` channel:

```
📢 React with 🔔 to get announcement notifications
💻 React with 💻 to get the Developer role
🐛 React with 🐛 to get the Bug Hunter role
```

---

## Step 7: Set Up Moderation

### 7.1 Moderation Rules

Configure in MEE6 or Discord AutoMod:

- Block offensive language
- Block spam (multiple messages quickly)
- Block excessive mentions (@everyone, @here)
- Block external links (except in appropriate channels)

### 7.2 Moderation Commands

Set up MEE6 commands:

- `!warn @user [reason]` - Warn a user
- `!mute @user [duration]` - Mute a user
- `!kick @user [reason]` - Kick a user
- `!ban @user [reason]` - Ban a user
- `!clear [number]` - Clear messages

### 7.3 Report System

Create a `#moderation` channel (visible only to moderators):

- User reports
- Moderation logs
- Ban appeals

---

## Step 8: Schedule Events

### 8.1 Weekly Office Hours

- **Time:** Choose a consistent day/time
- **Duration:** 1 hour
- **Channel:** 🎙️ Office Hours
- **Purpose:** Live Q&A, help, discussion

### 8.2 Monthly Hackathons

- **Date:** First weekend of each month
- **Duration:** 24-48 hours
- **Purpose:** Build features, fix bugs, create demos
- **Prizes:** Swag, contributor role, recognition

### 8.3 Release Parties

- **When:** New major releases
- **Purpose:** Celebrate releases, demo features
- **Activities:** Live demo, Q&A, giveaways

---

## Step 9: Create Invite Link

1. Go to Server Settings → Invites
2. Create invite with settings:
   - Max age: 7 days
   - Max uses: Unlimited
   - Grant temporary membership: No
3. Copy invite link
4. Add to website, README, social media

### Permanent Invite

Create a permanent invite for website/social media:
- Max age: Never
- Max uses: Unlimited

---

## Step 10: Promote the Server

### 10.1 Add to Website

Add Discord link to cookwithgasoline.com:

```html
<a href="https://discord.gg/YOUR_INVITE_CODE" target="_blank">
  <img src="discord-logo.png" alt="Join our Discord">
</a>
```

### 10.2 Add to README

Add to GitHub README.md:

```markdown
[![Discord](https://img.shields.io/badge/Discord-Join%20Server-5865F2.svg?logo=discord&logoColor=white)](https://discord.gg/YOUR_INVITE_CODE)
```

### 10.3 Social Media

Announce on Twitter/X, LinkedIn, Reddit:

```
🎉 We just launched the Gasoline MCP Discord community!

Join us for:
• Real-time support
• Feature discussions
• Weekly office hours
• Community events

🔗 https://discord.gg/YOUR_INVITE_CODE

#GasolineMCP #Discord #Community
```

---

## Step 11: Ongoing Management

### Daily Tasks
- [ ] Welcome new members
- [ ] Respond to help requests
- [ ] Moderate discussions
- [ ] Share updates in #announcements

### Weekly Tasks
- [ ] Host office hours
- [ ] Review bug reports
- [ ] Plan upcoming events
- [ ] Review moderation logs

### Monthly Tasks
- [ ] Host hackathon
- [ ] Review and update channels
- [ ] Analyze server analytics
- [ ] Recognize top contributors

---

## Step 12: Analytics

### Server Statistics

Use Statbot or Discord Insights to track:
- Member growth
- Active members
- Message count by channel
- Peak activity times

### Engagement Metrics

Track:
- New members per week
- Messages per day
- Active users (last 7 days)
- Voice channel usage

---

## Resources

- [Discord Server Guide](https://support.discord.com/hc/en-us/articles/204849977)
- [Discord Safety Center](https://discord.com/safety)
- [MEE6 Documentation](https://docs.mee6.xyz/)
- [Carl-bot Documentation](https://docs.carl.gg/)
- [Statbot Documentation](https://docs.statbot.net/)

---

## Next Steps

### This Week:
- [ ] Create Discord server
- [ ] Set up channels
- [ ] Configure bots
- [ ] Create welcome message
- [ ] Create invite link

### Next Week:
- [ ] Promote on social media
- [ ] Add to website
- [ ] Add to README
- [ ] Host first office hours

### Ongoing:
- [ ] Welcome new members
- [ ] Moderate discussions
- [ ] Host weekly events
- [ ] Recognize contributors

---

## Questions?

If you need help setting up your Discord server, join other developer communities or check Discord's documentation.

---

**Ready to create your community? Let's go! 🚀**
