# Issue Tracker: Local Markdown

Issues and specs (you may know a spec as a PRD) for this repo live as markdown files directly under `.issues/` at the repo root — a flat directory, not nested per-feature.

## Conventions

- One file per ticket: `.issues/<NNN>-<slug>.md`, three-digit zero-padded, numbered sequentially across the whole repo (not reset per feature/effort).
- Frontmatter on every ticket: `id`, `title`, `status`, `labels`, `assignee`. Wayfinder-managed tickets additionally carry `parent`, `blocks`, `blocked-by` (arrays of ticket ids as strings).
- `status` is `open` while live, `closed` once resolved.
- `assignee` is the claim mechanism: `null` means unclaimed; set to the claiming session/dev's name before starting work.
- The body holds a `## Question` heading (the ticket's question) and, once resolved, a `## Resolution` heading with the answer.
- Comments/conversation history, if any, append to the bottom of the file.

## When a skill says "publish to the issue tracker"

Create a new file at `.issues/<next-NNN>-<slug>.md` (zero-padded, next sequential id across the whole directory) with the frontmatter above.

## When a skill says "fetch the relevant ticket"

Read the file at `.issues/<NNN>-<slug>.md`. The user will normally pass the id or path directly.

## Wayfinding operations

Used by `/wayfinder`.

- **Map**: the map issue is a normal ticket in `.issues/`, labelled `wayfinder:map` (e.g. `.issues/001-map.md`) — the Destination / Notes / Decisions-so-far / Fog body.
- **Child ticket**: `.issues/NNN-<slug>.md` with `parent` set to the map's id, and a `wayfinder:<type>` label (`research`/`prototype`/`grilling`/`task`).
- **Blocking**: `blocks` / `blocked-by` frontmatter arrays of ticket ids. A ticket is unblocked when every id in `blocked-by` has `status: closed`.
- **Frontier**: scan `.issues/` for tickets that are `status: open`, unblocked, and `assignee: null`; lowest id wins.
- **Claim**: set `assignee` to the claiming session/dev, and save, before any work.
- **Resolve**: append the answer under a `## Resolution` heading, set `status: closed`, then append a context pointer (gist + link) to the map's Decisions-so-far.
