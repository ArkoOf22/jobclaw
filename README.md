# JobClaw

> An approval-gated, state-machine-driven job discovery and application automation system.

## Vision

JobClaw is a personal job-search operating system designed to automate repetitive work while
preserving human control over consequential decisions.

The goal is not to blindly apply to every job. The goal is to build a reliable system that can:

- discover jobs from multiple sources;
- filter irrelevant opportunities;
- score jobs;
- require approval before progressing;
- create applications;
- generate truthful, tailored resumes;
- understand application forms;
- ingest and classify questionnaires;
- resolve answers from verified candidate data;
- prepare submission payloads;
- validate readiness;
- submit through supported platform adapters;
- record submission outcomes;
- recover safely from failures and ambiguous external results.

The north star is:

> Automate repetitive work, preserve human control, never invent candidate information, and make
> consequential state transitions observable and recoverable.

---

## Design Principles

### Human approval before consequential actions

Job discovery can be automated. Application progression is deliberately gated.

```text
Discover → Score → Approve → Create → Prepare → Validate → Submit
```

### Preserve candidate truth

Resume tailoring may reword, reorder, and select relevant information according to configuration, but
the architecture is intended to prevent invented experience and facts.

### Separate preparation from execution

Preparing an application is different from externally submitting it.

```text
Application preparation ≠ External submission
```

Prepared data can therefore be validated and inspected before side effects occur.

### Treat external systems as unreliable

Network and ATS failures can leave an application outcome unknown. JobClaw models attempts, failures,
recovery, and ambiguous outcomes instead of assuming every submission is simply success or failure.

### Make state explicit

Important workflow transitions are persisted and represented explicitly.

### Make automation observable

Repositories, events, prepared data, submission attempts, errors, and tests make the workflow
inspectable.

---

# End-to-End Architecture

```text
                         JOB SOURCES
                  ┌──────────┼──────────┐
                  │          │          │
                JobSpy   Greenhouse   Future
                  │          │          │
                  └──────────┼──────────┘
                             ▼
                  ┌──────────────────────┐
                  │  Discovery Service   │
                  │ Concurrent execution │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │   Shared Matcher     │
                  │ Roles / Skills       │
                  │ Location / Remote    │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │    Job Repository    │
                  │       SQLite         │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │       Scoring        │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │    Approval Gate     │
                  │  Human-controlled    │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │ Application Service  │
                  └──────────┬───────────┘
                             ▼
              ┌──────────────┴──────────────┐
              ▼                             ▼
     Resume Generation              Form / Questionnaire
     Master resume                  Greenhouse forms
     Prompt builder                 Question extraction
     LLM / OpenRouter               Field classification
              └──────────────┬──────────────┘
                             ▼
                  ┌──────────────────────┐
                  │  Answer Resolution   │
                  │ Verified answer bank │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │ Prepared Submission  │
                  │        Data          │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │ Readiness Validation │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │ Submission Routing   │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │ Greenhouse Adapter   │
                  └──────────┬───────────┘
                             ▼
                  ┌──────────────────────┐
                  │ Attempt / Recovery   │
                  │ Success / Failure /  │
                  │ Ambiguous outcome    │
                  └──────────────────────┘
```

---

# What Has Been Built

## 1. Job Discovery

The discovery layer is based on a common source abstraction:

```go
type Source interface {
    Name() string
    Discover(ctx context.Context, request Request) ([]job.Job, error)
}
```

Current discovery capabilities:

- JobSpy integration;
- Greenhouse integration;
- multiple Greenhouse boards;
- Greenhouse board metadata/company resolution;
- full job content fetching;
- common job representation;
- repository upsert;
- concurrent source execution;
- source failure isolation.

The discovery service runs sources concurrently. A failure from one source does not prevent other
sources from returning results.

---

## 2. Shared Job Matcher

Matching logic has been extracted into:

```text
internal/discovery/matcher.go
```

It handles:

- role matching;
- skill/technology matching;
- multi-word role intent;
- punctuation normalization;
- location filtering;
- remote opportunities;
- remote-only requests.

The matcher deliberately avoids false positives where a role phrase appears only incidentally in a
description.

Examples of behavior tested include:

- Backend Engineer matches Backend Engineer;
- Golang can match title or description;
- Engineering Manager does not incorrectly match Backend Engineer;
- punctuation does not break token matching;
- location mismatches are filtered;
- remote jobs can satisfy appropriate location preferences;
- remote-only requests reject non-remote jobs.

---

## 3. Concurrent Discovery

Discovery sources now execute concurrently.

Conceptually:

```text
Source A ──┐
Source B ──┼── concurrent execution ──→ results
Source C ──┘
```

Benefits:

- faster network-bound discovery;
- one failing source does not stop the others;
- deterministic result ordering;
- source duration tracking.

Tests also use a concurrency-safe fake repository.

---

## 4. Job Persistence and Lifecycle

The job layer provides persistent internal identity independent of external platforms.

Supported concepts include:

- source and external ID lookup;
- internal ID lookup;
- listing;
- upsert;
- status updates.

---

## 5. Scoring and Company Intelligence Foundation

The repository includes:

```text
internal/scoring/
internal/company/
```

These provide the foundation for evaluating opportunities and company-related information before
application creation.

The intended flow is:

```text
Discover → Filter → Score → Review → Approve
```

---

## 6. Approval-Gated Applications

Applications are created for approved jobs.

This is an important architectural boundary:

```text
Discovery does not automatically equal application.
```

Approval is the control point before expensive or consequential application work proceeds.

---

## 7. Resume Generation

The application flow includes:

- master resume source;
- resume prompt builder;
- OpenRouter-backed LLM client;
- LLM resume generator;
- configuration-driven generation rules.

Configuration models controls around:

- preserving facts;
- rewording;
- reordering;
- skill selection;
- metric changes;
- experience invention.

The intended policy is:

> Tailor presentation, not reality.

---

## 8. Questionnaire Ingestion

A questionnaire pipeline has been implemented to turn raw application questions into structured
information.

Completed work includes:

- questionnaire ingestion;
- question/field classification;
- improvements to classification;
- completion of the ingestion pipeline.

This provides the basis for handling arbitrary application forms systematically.

---

## 9. Candidate Answer Bank

JobClaw includes a candidate answer repository.

Answers contain concepts such as:

- field key;
- answer;
- value type;
- verification state.

CLI operations exist for:

```text
answer add
answer update
answer list
```

Verification is important: the system can distinguish trusted candidate information from information
that should not be blindly reused.

---

## 10. Application Preparation and Readiness

The application workflow distinguishes:

```text
Application created
        ↓
Application prepared
        ↓
Application ready
        ↓
Submission
```

This prevents the system from treating a partially prepared application as ready for an external
side effect.

---

## 11. Prepared Submission Data

A dedicated prepared submission data layer exists.

Conceptually:

```text
Application
+ Resume
+ Form requirements
+ Resolved answers
        ↓
Prepared Submission Data
```

Benefits:

- inspectability;
- validation;
- debugging;
- adapter-specific transformation;
- safer retries.

---

## 12. Submission Architecture

The submission system has been developed through several milestones:

- atomic application submission flow;
- submission attempt state and recovery;
- ambiguous submission handling;
- submission adapter routing;
- prepared submission data;
- Greenhouse submission adapter.

This is designed to handle the fact that external systems are not perfectly reliable.

For example:

```text
Request sent
    ↓
Remote system may accept
    ↓
Network response lost
    ↓
Local system cannot safely know outcome
```

JobClaw therefore treats ambiguity as a real state instead of blindly retrying and risking duplicate
applications.

---

## 13. Greenhouse Discovery

The Greenhouse discovery implementation now:

1. receives a board token;
2. fetches board metadata;
3. determines the company name;
4. fetches jobs with full content;
5. transforms jobs into the common domain model;
6. filters jobs through the shared matcher.

Multiple Greenhouse boards are supported, and empty board tokens are ignored.

---

## 14. Greenhouse Form Automation

The latest major milestone added:

- Greenhouse URL handling;
- URL parsing/normalization;
- Greenhouse application form domain model;
- form provider abstraction;
- HTTP-based form provider;
- form parsing;
- form validation;
- integration with the application preparation flow.

This moves Greenhouse beyond simple job discovery toward understanding the application itself.

---

## 15. Greenhouse Submission

A dedicated Greenhouse submission adapter exists behind the platform-specific submission
architecture.

The intended model is:

```text
Prepared application
        ↓
Submission router
        ↓
Greenhouse adapter
        ↓
External submission
        ↓
Attempt state recorded
```

Future platforms should follow the same architectural boundary instead of putting platform-specific
behavior inside the core application service.

---

# Repository Architecture

```text
jobclaw/
│
├── cmd/
│   └── jobclaw/
│       └── main.go              # CLI composition root
│
├── internal/
│   │
│   ├── application/
│   │   ├── application lifecycle
│   │   ├── resume generation
│   │   ├── questionnaire ingestion
│   │   ├── candidate answer bank
│   │   ├── preparation/readiness
│   │   ├── prepared submission data
│   │   ├── submission attempts
│   │   ├── recovery
│   │   ├── Greenhouse forms
│   │   └── Greenhouse submission
│   │
│   ├── company/
│   │   └── company classification
│   │
│   ├── config/
│   │   └── configuration
│   │
│   ├── database/
│   │   └── persistence infrastructure
│   │
│   ├── discovery/
│   │   ├── request
│   │   ├── source abstraction
│   │   ├── concurrent service
│   │   ├── shared matcher
│   │   ├── greenhouse/
│   │   │   ├── client
│   │   │   └── multi-board support
│   │   └── jobspy/
│   │
│   ├── job/
│   │   └── job domain and repository
│   │
│   ├── llm/
│   │   └── openrouter/
│   │
│   └── scoring/
│       └── job scoring
│
├── data/
│   └── applications/
│
├── go.mod
├── go.sum
└── README.md
```

---

# CLI Role

The CLI is currently the composition root.

It wires together concrete implementations such as:

```text
SQLite repositories
        +
LLM client
        +
Resume generator
        +
Form provider
        +
Submission adapter
        ↓
Application services
```

This keeps business logic in domain packages and infrastructure/platform-specific code behind
explicit boundaries.

---

# Reliability Model

JobClaw has deliberately prioritized correctness.

Important properties include:

## Context and timeouts

Operations use `context.Context`, with timeouts appropriate to their workflows.

## Atomic submission flow

Submission state is modeled explicitly instead of relying on a single fire-and-forget operation.

## Recovery

Interrupted or failed workflows can be represented and recovered.

## Ambiguous outcomes

Unknown external outcomes are not silently converted into success or failure.

## Source isolation

One failed discovery provider does not stop the remaining providers.

---

# Testing and Quality

The project has been continuously validated with:

```bash
go test ./...
go vet ./...
```

At the latest checkpoint, both were clean.

Discovery tests cover:

- successful discovery;
- persistence;
- source failure isolation;
- concurrent execution;
- role matching;
- skill matching;
- punctuation normalization;
- false-positive prevention;
- location matching;
- remote behavior.

Greenhouse tests cover:

- discovery;
- board metadata;
- matching;
- multi-board behavior;
- empty board handling.

The application package also contains extensive tests around preparation, submission, forms, and
related lifecycle behavior.

---

# Completed Milestones

```text
35de9c7  Add questionnaire ingestion pipeline
01fb2d3  Improve questionnaire field classification
dd69f0f  Complete questionnaire ingestion pipeline

b702ee1  Add application preparation and readiness flow
715d51a  Add atomic application submission flow
b881fa6  Add submission attempt state and recovery
751fb63  Add ambiguous submission handling
bac495f  Add submission adapter routing
9820526  Add prepared submission data layer
33821f5  Add Greenhouse submission adapter

a5b0edd  Add Greenhouse discovery and application form automation
```

The latest documented checkpoint is:

```text
Commit: a5b0edd
Branch: main
Remote: origin/main
Working tree: clean
Full test suite: passing
go vet: passing
```

---

# What Remains

The project is not finished. The biggest remaining task is to prove the existing architecture as one
real vertical slice.

## Immediate priority: end-to-end execution

We need to verify:

```text
Discover
    ↓
Store
    ↓
Score
    ↓
Approve
    ↓
Create application
    ↓
Generate tailored resume
    ↓
Fetch Greenhouse form
    ↓
Extract/classify questions
    ↓
Resolve answers
    ↓
Prepare submission
    ↓
Validate readiness
    ↓
Submit
    ↓
Record outcome
```

The focus should be on finding missing orchestration between existing components rather than
prematurely building more isolated features.

---

# Future Work

## Answer resolution engine

The system should distinguish:

```text
Known question
    → use verified answer

Similar known question
    → map safely

Unknown question
    → require review

Ambiguous question
    → do not guess
```

## Real-world integration validation

Unit tests need to be complemented by carefully controlled validation against real supported systems.

## Resume artifact hardening

Continue improving:

- output validation;
- artifact storage;
- application association;
- submission compatibility.

## Additional ATS adapters

Potential future targets include:

```text
Lever
Ashby
Workday
SmartRecruiters
Other supported systems
```

These should plug into the existing adapter boundaries.

## Better company intelligence

Potential signals:

- company stage;
- funding;
- size;
- industry;
- hiring velocity;
- role quality;
- engineering fit.

## Operational dashboard/reporting

Eventually expose:

```text
Jobs discovered
High-scoring jobs
Jobs awaiting approval
Applications being prepared
Ready applications
Submitted applications
Failed attempts
Ambiguous attempts
Items requiring attention
```

## Scheduling

The eventual automated discovery loop could be:

```text
Scheduled run
    ↓
Discover jobs
    ↓
Deduplicate/upsert
    ↓
Match and score
    ↓
Present best opportunities
    ↓
Wait for approval
```

---

# Roadmap

## Phase 1 — Core Foundation

**Status: Completed**

- job domain;
- persistence;
- configuration;
- lifecycle foundations;
- scoring foundations.

## Phase 2 — Discovery

**Status: Strong foundation completed**

- source abstraction;
- JobSpy;
- Greenhouse;
- multi-board discovery;
- shared matching;
- concurrency.

## Phase 3 — Application Preparation

**Status: Largely completed**

- application creation;
- resume generation;
- questionnaires;
- answer bank;
- preparation;
- readiness.

## Phase 4 — Greenhouse Vertical Slice

**Status: Core implementation completed**

- discovery;
- board metadata;
- forms;
- URL handling;
- submission adapter.

**Next:** prove the complete real-world flow.

## Phase 5 — Operational Automation

**Planned**

- scheduled discovery;
- approval queues;
- retries/recovery;
- reporting;
- monitoring.

## Phase 6 — Multi-Platform Support

**Planned**

- additional ATS adapters;
- additional discovery sources.

## Phase 7 — Full Job-Search Operating System

**Long-term**

```text
Search
→ Discover
→ Filter
→ Understand
→ Score
→ Approve
→ Tailor
→ Answer
→ Prepare
→ Validate
→ Apply
→ Track
→ Recover
→ Improve
```

---

# How to Read the Codebase

A recommended reading order:

## 1. CLI

```text
cmd/jobclaw/main.go
```

Understand how dependencies are wired.

## 2. Job domain

```text
internal/job/
```

Understand jobs, repositories, and status transitions.

## 3. Discovery

```text
internal/discovery/source.go
internal/discovery/request.go
internal/discovery/service.go
internal/discovery/matcher.go
```

Then inspect:

```text
internal/discovery/jobspy/
internal/discovery/greenhouse/
```

## 4. Scoring and company logic

```text
internal/scoring/
internal/company/
```

## 5. Application workflow

Read `internal/application/` as a pipeline:

```text
Application
→ Resume
→ Questionnaire
→ Answers
→ Preparation
→ Readiness
→ Submission
→ Attempt state
→ Recovery
```

## 6. Greenhouse integration

Study the Greenhouse implementation as the reference for platform-specific architecture.

The core principle is:

> Platform-specific complexity should live behind platform-specific boundaries while the core
> application workflow remains platform-independent.

---

# Final Goal

JobClaw should eventually behave like this:

```text
JobClaw discovers opportunities
        ↓
Filters obvious noise
        ↓
Evaluates relevance
        ↓
Candidate approves important actions
        ↓
Prepares truthful, tailored application material
        ↓
Understands supported application forms
        ↓
Resolves known answers safely
        ↓
Surfaces unknown/risky questions
        ↓
Validates readiness
        ↓
Submits through supported adapters
        ↓
Records exactly what happened
        ↓
Recovers safely from uncertainty
```

JobClaw is not intended to be a blind mass-application bot.

It is intended to be a **reliable, extensible, state-aware job application system** that reduces
repetitive work while maintaining control, correctness, transparency, and candidate truth.

---

## Current Development Priority

**Prove one complete end-to-end vertical slice before expanding further.**

The immediate target is:

```text
Discovery
→ Storage
→ Scoring
→ Approval
→ Application
→ Resume
→ Form
→ Questions
→ Answers
→ Preparation
→ Readiness
→ Submission
→ Outcome
```

Once this complete flow is validated, the architecture will have a strong foundation for expanding to
additional platforms, scheduling, richer intelligence, and a full personal job-search operating system.
