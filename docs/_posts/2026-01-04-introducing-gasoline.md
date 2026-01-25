layout: post
title: "Introducing Gasoline: AI Fueled Web Dev"
date: 2026-01-04 10:00:00 +0000
author: Brenn Hill
categories: [announcement, ai, devtools]


<p align="center">
	<img src="/assets/images/sparky/sparky-holds-browser-web.webp" alt="Sparky holding a browser" width="320" />
</p>

# Your AI Assistant Can't See Your Browser

Let's be honest: AI coding assistants are cool, but they're still pretty clueless about what's actually happening in your browser. There are tools like Chrome MCP, but they are actually quite limited. If you've ever pasted a stack trace into ChatGPT or Claude and then spent five minutes explaining what just happened, you know the pain. 


<p align="center">
	<img src="/assets/images/sparky/sparky-fight-fire-web.webp" alt="Sparky fighting fire" width="300" />
</p>

Gasoline wants to make this problem die in a fire. Gasoline is fuel for the AI Fire.


## Why Is This Still a Thing?

We keep pretending these tools are omniscient. They're not. They see your code, but not your runtime. They don't know about the network request that just failed, or the DOM node that went missing, or the weird CSS bug that only shows up on your machine.

So we copy, we paste, we explain, we lose context. It's not just slow - it's lossy. The AI is guessing, asking you to do this and that… And the actual debugging? That's the easy part.

<p align="center">
	<img src="/assets/images/sparky/sparky-confused-dizzy-web.webp" alt="Sparky confused" width="220" />
</p>

## The Copy-Paste Cycle Tax

Every time you copy an error, describe a UI glitch, or try to explain a network failure, you're losing information. The AI is working with half the story. You're doing the translation work. And the whole process is slower than it should be. Copy, paste, screenshot, query, copy, paste… blah blah kill me.

<p align="center">
	<img src="/assets/images/sparky/sparky-confused-skillet-web.webp" alt="Sparky with a skillet" width="220" />
</p>

## What If Your AI Could Actually See?

That's why I built Gasoline. It's open source, and it just sits in the background, streaming your browser's runtime data to your AI - console logs, network errors, DOM state, the works. No weird setup, no cloud, no telemetry. Everything stays local. You work the way you always have, but your AI finally gets the full picture.

<p align="center">
	<img src="/assets/images/sparky/sparky-working-laptop-web.webp" alt="Sparky working on a laptop" width="220" />
</p>

## Not Magic, Just Useful

This isn't some AGI moment. It's just a tool that closes the gap between your browser and your AI. You get faster feedback, less context-switching, and your assistant actually knows what's going on.

<p align="center">
	<img src="/assets/images/sparky/sparky-thumbs-up-web.webp" alt="Sparky thumbs up" width="180" />
</p>

If you want to hack on it, or just try it out, it's all on GitHub. No sales pitch, no lock-in, no vendor allegiance. Transparent and open source AGPL 3.0

Install Gasoline and help me improve AI driven development. Submission to Google chrome store is pending, but you can still install manually.

Or check out the code at [github.com/brennhill/gasoline](https://github.com/brennhill/gasoline)

I'll be putting together some tutorial and demo videos of v5 shortly.

<p align="center">
	<img src="/assets/images/sparky/sparky-grill-web.webp" alt="COOK WITH GASOLINE" width="340" />
	<br><b>COOK WITH GASOLINE</b>
</p>

---

From Brenn Hill - building tools for devs who want to move faster, not just automate busywork. Sparky the Salamander approves this message.
