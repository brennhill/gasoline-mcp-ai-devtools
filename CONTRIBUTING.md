# Contributing to Gasoline

Thanks for your interest in contributing! We appreciate all kinds of contributions.

## Easy Ways to Contribute

### 1. **Report Issues**
Found a bug? Have a feature idea? [Open an issue](https://github.com/brennhill/gasoline-mcp-ai-devtools/issues). Be specific about:
- What you were trying to do
- What happened vs what you expected
- Your setup (OS, Chrome version, Node.js version)

### 2. **Share Feedback**
- Use Gasoline and tell us what's working (or not)
- [GitHub Discussions](https://github.com/brennhill/gasoline-mcp-ai-devtools/discussions) are great for ideas
- [X/Twitter](https://x.com/gasolinedev) for quick feedback

### 3. **Improve Documentation**
- Found a typo or unclear explanation? [Edit docs](docs/)
- Add examples to guides
- Fix broken links

### 4. **Star the Project**
If Gasoline helps you, [star the repo](https://github.com/brennhill/gasoline-mcp-ai-devtools/stargazers). It helps others discover it.

## Code Contributions

### Setup
```bash
git clone https://github.com/brennhill/gasoline-mcp-ai-devtools.git
cd gasoline
make install
make test
```

### Before You Code
1. Check [existing issues](https://github.com/brennhill/gasoline-mcp-ai-devtools/issues) and [PRs](https://github.com/brennhill/gasoline-mcp-ai-devtools/pulls)
2. Open an issue to discuss larger changes
3. Keep changes focused (one feature per PR)

### Standards
- **TDD**: Write tests first
- **No breaking changes**: Extensions rely on our APIs
- **TypeScript strict mode**: No `any` types
- **Zero external deps**: Keep production dependencies minimal
- **Run `make test`** before submitting

### What We Review
✅ Bug fixes
✅ Performance improvements
✅ Documentation
✅ Test coverage
✅ Security hardening

❌ Major API changes (discuss first)
❌ New external dependencies
❌ Unrelated refactoring

## Questions?

- 📖 [Docs](https://cookwithgasoline.com)
- 💬 [GitHub Discussions](https://github.com/brennhill/gasoline-mcp-ai-devtools/discussions)
- 🐛 [Issues](https://github.com/brennhill/gasoline-mcp-ai-devtools/issues)

## Developer Certificate of Origin (DCO)

All contributions require a DCO sign-off. Every commit in your pull request must
include a `Signed-off-by` line certifying that you wrote the code (or have the
right to submit it) and agree to the terms below. This is enforced by CI.

Sign off by adding `-s` to your commits:

```bash
git commit -s -m "Your commit message"
```

To fix existing commits:

```bash
git rebase --signoff HEAD~N    # sign-off the last N commits
```

The sign-off certifies the [Developer Certificate of Origin (v1.1)](https://developercertificate.org/).

## Intellectual Property and License

By submitting a contribution (including, without limitation, any code, documentation,
or other materials) to this project, you hereby irrevocably assign to Brenn Hill all
right, title, and interest worldwide in and to such contribution, including all
intellectual property rights therein. You acknowledge and agree that Brenn Hill shall
have the unrestricted right to use, reproduce, modify, distribute, sublicense, and
otherwise exploit the contribution in any manner and for any purpose, under any license
terms Brenn Hill may select, at his sole discretion.

You represent and warrant that: (a) you are the sole author of the contribution and
have the legal right to make the foregoing assignment; (b) the contribution does not
infringe upon the intellectual property rights of any third party; and (c) if the
contribution was created in the course of employment, you have obtained any necessary
permissions from your employer to make this assignment.

The DCO sign-off on each commit constitutes your acceptance of these terms. The project
is currently distributed under the AGPL-3.0 license. Nothing in this section limits
Brenn Hill's right to relicense, dual-license, or otherwise change the licensing terms
of the project or any contribution at any time.

---

**We're grateful for every contribution.** Even small improvements make Gasoline better for AI developers everywhere.
