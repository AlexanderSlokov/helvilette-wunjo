# Terms

- **Founder**: The person who has the final say on all decisions, aka @AleksandrSlokov. You can check the git email/username in the git commits to know who you 're talking to.

# Workflow instructions

- When new ideas arrive from the Founder, update `ROADMAP.md`.
- After your plan is accepted, update `BACKLOG.md` to propagate task items. Other AI Agents may pick them up to do.
- After finished feature implement / bug fix,... Update the `CHANGELOG.md`.

## Documentation flow

A change moves through the following documents in order:

1. `ROADMAP.md`: A new capability appears here first, attached to a phase
   and a target version. Changes must decend from here to BACKLOG, 
   except corrections to artifacts that already exist (a wrong figure, a stale path).

2. `BACKLOG.md`: Roadmap items decompose into tasks here. Each task
   names the files it touches, its dependencies on other tasks, and an explicit
   "goal" condition. Group tasks by the artifact they modify. Link the GitHub
   issue when one exists. The issue holds the discussion, the backlog item holds the
   plan of record.

3. `CHANGELOG.md`: When a backlog task is complete, remove it from
   `BACKLOG.md` and describe the result under `[Unreleased]`, following Keep a Changelog.
   They will be release notes verbatim, so write short and concise: what changed and why it mattered.

## Documentation layout

They are in `docs/` folder, follows [Diátaxis](https://diataxis.fr/):

- `docs/tutorials/` — learning-oriented. A beginner following steps to a first result.
- `docs/how-to/` — task-oriented. A competent user achieving one specific goal.
- `docs/references/` — information-oriented. Facts to look up: schemas, measured
  numbers, the paper, session records.
- `docs/explanation/` — understanding-oriented. Designs, ideas, and rationale.

`docs/plans/` holds artifacts, like implementation plans.


## Writing documentation

These rules apply to every documentation files (every `.md` files) in this repository. They adapt the
[Kubernetes documentation style guide](https://kubernetes.io/docs/contribute/style/style-guide/)
and the Kubernetes [page content types](https://kubernetes.io/docs/contribute/style/page-content-types/)
to a repository that GitHub renders. In case of conflict, follow this document.

### Write for a reader who is new to the subject

Assume the reader can use a terminal and has never heard of this project. English may be
their second language. A page works when that reader can understand what it describes on the
first attempt.

- Near the top of the page, answer four questions: what this is, why the reader would
  want it, how to do it, and how to tell that it worked.
- Define each term the first time it appears. Put the term in italics and follow it
  with a plain explanation: "A _blueprint_ is a YAML file that maps each Pod to a
  node."
- Give a concrete example for each abstract statement: a real command, a real file, or
  a real number.
- After a command, show what the reader should see. Introduce the output with "The
  output is similar to this:".
- Cover one idea per paragraph, and keep paragraphs under six sentences.
- When a step needs background, link to the page that explains it. Each fact lives in
  one place, so it has one place to update.

### Choose the page type first

Each page in `docs/` has one purpose, and the purpose decides its location and its
sections. Write the sections in the order shown:

| Type | Purpose | Location | Sections |
|---|---|---|---|
| Tutorial | Takes a beginner to a first complete result | `docs/tutorials/` | Overview · Objectives · Before you begin · Steps, grouped under H2 headings · Clean up · What's next |
| How-to | Shows a competent user how to do one task | `docs/how-to/` | Overview (what and why) · Before you begin · Steps · Discussion (optional) · What's next |
| Explanation | Explains why the design is the way it is | `docs/explanation/` | Overview · Body under H2 and H3 headings · What's next |
| Reference | States facts to look up: schemas, flags, measured numbers | `docs/references/` | Description · Tables, schemas, or results |

The overview is one paragraph that sets context. Steps are numbered lists. An
explanation page contains no steps and links to the how-to page that has them. A
reference page describes and contains no instructions. "What's next" lists at most five
links.

### The repository documents

The top-level documents serve:

| Document | Reader | Shape |
|---|---|---|
| `README.md` | Someone deciding whether to use this project | The Kubernetes [Overview](https://kubernetes.io/docs/concepts/overview/) page: what it is, why you need it, what it provides, how it works, before you begin, a working example, and what This project is not. |
| `ROADMAP.md` | Everyone | Per item: one sentence of intent, a link to the plan, and one-line bullets. Reasoning and figures go in the plan. |
| `BACKLOG.md` | Agents and contributors | Per task: ID, task, files touched, dependencies, and the done-when condition. A table is enough. |
| `CHANGELOG.md` | Operators | Per entry: a bold sentence that states the change, then what it means for the operator. Link the issue. |
| Session records | Whoever checks a number later | Commit, command, host, and a results table, as described in [Session records](#session-records). |
| `docs/plans/` | Agents that execute the work | Stages, each with a done-when condition. Reasoning is welcome here. |
| ADRs | Contributors | Context, decision, and consequences. |

### Language

The following table shows how to write a sentence:

| Do | Don't |
|---|---|
| This command writes a blueprint. | This command will write a blueprint. |
| The solver reads the infrastructure file. | The infrastructure file is read by the solver. |
| You can reserve capacity with `--headroom`. | We can reserve capacity with `--headroom`. |
| Version 0.3.0 adds pod groups. | In version 0.3.0, we added pod groups. |
| To plan a cluster, run the solver. | In order to plan a cluster, simply run the solver. |
| For example, a GPU node … | E.g., a GPU node … |
| Internally, the solver … | Under the hood, the solver … |
| In version 0.3.0, … | Currently, … / The new … |
| This project reads usage statistics from a file. | This project reads usage statistics as a file, never by polling. |
| One plan of MSC Irina took 185 seconds at commit `24ffc52`. | This project computes in seconds. |
| The inspector checks every constraint with its own code. | Mathematically grounded and infinitely more defensible. |

Use present tense, active voice, and "you" for the reader. 
Use U.S. English spelling.
Use the passive voice only when the active voice reads awkwardly.

Three documents have exceptions. `PAPER.md` follows academic convention and may use
"we". `ROADMAP.md` describes planned work. Session records may use the passive voice to
report what was measured.

### Patterns to avoid

- **Contrastive framing.** Sentences built as "X, not Y", "never Y", "rather than Y",
  or "neither … nor" to add emphasis. State what the thing is or does. A plain negative
  fact stays: "The solver does not connect to the cluster."
- **Words that assume ease or knowledge:** just, simply, easy, easily, obviously, of
  course.
- **Claims without a measurement.** Each number names the commit and the session record
  that produced it. A qualitative claim about speed or quality needs a number behind it.
- **Promises about features that do not exist.** Planned work belongs in `ROADMAP.md`.
- **Rhetorical questions, exclamation marks, and emoji.**
- **Bold text used as a heading or for emphasis in prose.** Bold marks a UI element or
  the lead term of a list item.
- **Acronyms without a definition.** Spell out an acronym at first use: "first-fit
  decreasing (FFD)".

### Markdown formatting

- **Headings.** The H1 is the page title. Use sentence case for H2
  and below: "Plan a deployment". Keep the levels in order, avoid H4 to H6, and keep
  links out of headings.
- **Line length.** Wrap lines at about 88 characters, at the end of a sentence or after
  a punctuation mark. Keep each Markdown link on one line. Diffs then show the sentence
  that changed.
- **Code style.** Use backticks for file names, paths, commands, flags, field names,
  `make` targets, and tool and component names (`This project-solver`, `kube-scheduler`).
- **Tool names at the start of a sentence.** Lead with a descriptor: "The
  `This project-forge` tool converts manifests."
- **Kubernetes API kinds.** Write them in UpperCamelCase without backticks: Pod,
  DaemonSet, ConfigMap.
- **Field values.** Write string and integer values in plain text: "Set `replicas` to
  4."
- **Placeholders.** Use angle brackets, and say what the placeholder stands for after
  the block: "where `<infra-file>` is the path to your infrastructure document."
- **Emphasis.** Use italics for a term where you define it. Use bold for UI elements.
- **Lists.** Use `1.` for every item of an ordered list and `-` for unordered lists. If
  one item is a full sentence, end every item with a period. Indent nested content by
  four spaces. Introduce each list and each table with a sentence.
- **Links.** Use relative paths inside the repository. Make the link text name the
  target: "See [This project IR v1](docs/references/ir-v1.md)." Avoid "here" as link text.
- **Callouts.** Use GitHub alerts sparingly. `> [!NOTE]` holds information that helps.
  `> [!CAUTION]` marks a pitfall. `> [!WARNING]` marks a risk of data loss or damage to
  a cluster.
- **Code blocks.** Fence every block and name its language: `shell` for commands,
  `console` or no language for output, `yaml` for files. Leave out the `$` prompt. Put
  commands and their output in separate blocks. Use meaningful names in examples, such
  as `gpu-node-01`.
- **Numbers.** Use thousands separators (2,714 Pods) and name the unit (185 seconds).
- **Diagrams.** Use a fenced `mermaid` block, followed by a caption line such as
  "Figure 1. The planning pipeline." The surrounding text carries the full meaning, so
  a reader who cannot see the diagram loses nothing.

### Run every example

Run each command in a document at the commit you are documenting, before you commit
the document. A command that fails costs the reader more than a missing one. The
README once showed `This project-forge in` with its flags after the source paths. The Go
flag parser stops at the first source, so the command exited with "nothing to do".

If a command needs a tool, the "Before you begin" section names it.

### Check a document before you commit it

1. Confirm that the page type is right and that its sections appear in order.
1. Confirm that each term and acronym is defined at first use.
1. Run each command, and show its output where the reader needs it.
1. Confirm that each number cites a commit and a session record.
1. Search for patterns to avoid:

    ```shell
    grep -nE "\b(we|we'll|will|just|simply|easily|currently|e\.g\.|i\.e\.)\b|, not |never |rather than" <file>
    ```

    where `<file>` is the document you changed. Each match is a sentence to reread. Some
    matches are correct, such as "will" in `ROADMAP.md`.

1. Confirm that each relative link points to a file that exists.

## Constraints

- Write every `*.md` artifact by the rules in [Writing documentation](#writing-documentation).

## Code style

- Functions: 4-20 lines. Split if longer.
- Files: under 500 lines. Split by responsibility.
- One thing per function, one responsibility per module (SRP).
- Names: specific and unique. Avoid `data`, `handler`, `Manager`.
  Prefer names that return <5 grep hits in the codebase.
- Types: explicit. No `any`, no `Dict`, no untyped functions.
- No code duplication. Extract shared logic into a function/module.
- Early returns over nested ifs. Max 2 levels of indentation.
- Exception messages must include the offending value and expected shape.

## Comments

- Keep your own comments. Don't strip them on refactor — they carry
  intent and provenance.
- Write WHY, not WHAT. Skip `// increment counter` above `i++`.
- Docstrings on public functions: intent + one usage example.
- Reference issue numbers / commit SHAs when a line exists because
  of a specific bug or upstream constraint.

## Tests

- Tests run with a single command: `make test`.
- Every new function gets a test. Bug fixes get a regression test.
- Mock external I/O (API, DB, filesystem) with named fake classes,
  not inline stubs.
- Tests must be F.I.R.S.T: fast, independent, repeatable,
  self-validating, timely.

## Dependencies

- Inject dependencies through constructor/parameter, not global/import.
- Wrap third-party libs behind a thin interface owned by this project.

## Structure

- Follow the framework's convention (Rails, Django, Next.js, etc.).
- Prefer small focused modules over god files.
- Predictable paths: controller/model/view, src/lib/test, etc.

## Formatting

- Use the language default formatter (`cargo fmt`, `gofmt`, `prettier`,
  `black`, `rubocop -A`). Don't discuss style beyond that.

## Logging

- Structured JSON when logging for debugging / observability.
- Plain text only for user-facing CLI output.