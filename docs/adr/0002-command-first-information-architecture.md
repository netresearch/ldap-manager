# 2. Command-first information architecture

Date: 2026-04-20

## Status

Accepted. Implemented in the UI revamp, Phase 1.

## Context

The directory has three entity kinds — users, groups, computers — that reference each other.
Two shapes were on the table for browsing them. A tree-first explorer mirrors the LDAP DIT and
shows containment directly. A command-first interface puts a search palette in front and treats
the lists as filtered views.

## Decision

The interface is command-first. A ⌘K command palette searches every entity kind, the list pages
carry filters and a detail drawer, and pivot links move between related entities. A tree-first
or dense three-pane explorer was rejected as the primary navigation.

## Consequences

- Finding an entity is a search, not a descent through OUs. This suits directories whose OU
  structure is deep or inconsistent, which is the common case.
- The palette needs a client-side search index, served from `/api/search-index.json`, and a
  place to keep recents and pins. Pins are per user and persist in a bbolt store beside the
  session store.
- Containment is visible through pivots and the relationship graph rather than through a
  permanent tree. A tree remains possible later as an optional view; it is not the spine of
  the interface.
- Every list page has to answer the palette's needs as well as its own, so entity metadata is
  built once in the search index and reused.

## Source

`docs/superpowers/specs/2026-04-20-ui-revamp-design.md` §5 and its rejected-alternatives note,
as of commit `da77c81`. The working specs and plans were removed from the tree once the work
shipped.
