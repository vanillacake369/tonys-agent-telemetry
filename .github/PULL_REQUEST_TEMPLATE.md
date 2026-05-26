<!--
Thanks for sending a PR. Filling out this template makes review faster
and lets the maintainer focus on the substance rather than checklist
hygiene. Anything you can't tick → leave it unchecked and explain why
in the description below.
-->

## What & why

<!-- One paragraph. Link the issue if there is one. -->

Closes #

## How

<!-- A sentence or two on the implementation approach. Mention any non-obvious tradeoffs. -->

## Test plan

<!-- How you verified the change. New tests added, manual smoke steps, screenshots/GIFs for UI changes. -->

## Checklist

- [ ] Tests added or updated (race-clean: `make test-race`)
- [ ] Docs updated (`README.md` and/or relevant `docs/*.md`)
- [ ] `make ci` passes locally
- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
      (enforced by `.githooks/commit-msg` once `make hooks-install` is run)
- [ ] No new external Go dependencies (or: I've explained why one is needed)

## AI-assistance disclosure

<!--
Per CONTRIBUTING.md: AI pair-programming is welcome, just tell reviewers
roughly how it was used so they can focus their scrutiny.
Examples:
  - "Hand-written; no AI."
  - "Claude drafted the detector; I rewrote the hashing logic + added bench."
  - "GPT-4 generated the test fixtures only."
-->
