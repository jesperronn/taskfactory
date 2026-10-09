# TF-040: AI prose tells lint

## Purpose

TF-040 adds an advisory lint for mechanical AI-style phrasing. It uses
mdsmith v0.57.0 rules MDS056 (forbidden-text) and MDS055
(forbidden-paragraph-starts), configured in `.mdsmith-tells.yml`. The default
`bin/lint` and `.mdsmith.yml` are unchanged. `bin/lint --tells` runs only the
advisory config over `docs/`, `README.md`, `tasks/inbox/`, `tasks/ready/`,
`tasks/active/` and `tasks/failed/`, prints the findings and a stats line, and
exits 0. Usage errors still exit 2. The archive is out of scope.

Both rules match case-sensitively, so each phrase is listed in the lower case
and the capitalised form that starts a sentence. The rules read paragraph text
only, so code fences, tables and headings are never matched.

## The lists

Before choosing, the repository was searched for each candidate. None of the
chosen phrases occurs in the scoped files, and none is a domain term of the
project (tasks, workers, claim, verify, integrate, evidence, lint).

Forbidden text (MDS056):

| Phrase                         | Rationale                                  |
| ------------------------------ | ------------------------------------------ |
| delve                          | Stock verb of model prose; "look at" works |
| it is / it's important to note | Filler that announces a point, adds none   |
| it is / it's worth noting      | Same filler, softer form                   |
| in today's fast-paced          | Stock scene-setting opener                 |
| seamless                       | Vague marketing adjective, not testable    |
| robust solution                | Vague praise; name the failure it survives |
| leverage                       | Inflated verb for "use"                    |
| navigate the complexities      | Stock metaphor with no content             |
| rich tapestry                  | Stock metaphor with no content             |
| game-changer                   | Hype word, not a specification             |

Forbidden paragraph starts (MDS055): "In conclusion,", "In summary,",
"Certainly!", "Great question", "Overall,", "Moreover,", "Furthermore," and
"Additionally,". These are chat-style openers and essay connectives that
contracts and documentation do not need; each statement in a task contract
stands alone.

Words such as "ensure", "simply" and "ecosystem" were considered and left out:
they occur in ordinary technical prose and would be false positives.

## Run

Command: `bin/lint --tells`

```text
stats: tells MDS055=0 MDS056=0
```

MDS055: 0
MDS056: 0

## Review of findings

Every finding is reviewed by hand and classified as a true tell to reword, a
false positive to allow for by narrowing the lists, or a domain term kept.

| File | Phrase | Class | Action |
| ---- | ------ | ----- | ------ |
| none | none   | n/a   | none   |

true tell: 0
false positive: 0
domain term kept: 0
unclassified: 0

No prose was reworded: the in-scope backlog was already clean, so there is no
rewording commit. The tell findings stay out of the default `bin/lint` exit
code. Tests for the mode are in `bin/lint.tells.test.sh`.
