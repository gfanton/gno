# Writing documentation that AI agents actually understand

**The documentation your agents read is the single biggest lever on their performance.** Anthropic's engineering team found that "even small refinements to tool descriptions can yield dramatic improvements" — Claude Sonnet 3.5 achieved state-of-the-art performance on SWE-bench Verified after precise description refinements that dramatically reduced error rates. Yet most developers still write tool descriptions as an afterthought, treating them like code comments rather than the critical interface contracts they are. This guide synthesizes best practices from Anthropic, OpenAI, Google, production MCP deployments at Block (60+ servers, 4,000 users), Microsoft Learn's public MCP server, and the broader agent tooling community into a practical framework for writing documentation that makes agents genuinely more capable.

---

## Agents don't read like humans — they parse like compilers with intuition

The shift from human-readable to agent-readable documentation requires a fundamental reframe. As Stytch's engineering team put it: "For an agent, docs must be complete, unambiguous, and centralized — otherwise the model treats missing or conflicting information as fact." This captures the core difference: **humans infer context; agents confabulate it**.

Several key asymmetries define this gap. Humans tolerate inconsistency — using `user_id` in one endpoint and `userId` in another is a minor annoyance for a developer, but an agent will treat these as unrelated fields and generate broken code. Humans skim selectively — they jump to the section they need. Agents consume tokens linearly, meaning every CSS class, navigation element, and sidebar widget wastes context budget. Humans can Google when they're stuck; agents that stall have hit a documentation failure.

Anthropic's engineering blog frames this philosophically: "Tools are a new kind of software which reflects a contract between deterministic systems and non-deterministic agents." When a user asks "Should I bring an umbrella today?", an agent might call a weather tool, answer from general knowledge, or ask a clarifying question about location first. This nondeterminism means your documentation isn't just a reference — it's the decision-making substrate that shapes which path the agent takes.

Docker's documentation team identified a critical nuance: **you're serving two audiences simultaneously**. The end user decides which tools the agent can access. The agent decides which tools to actually invoke. Your tool names, descriptions, and schemas must satisfy both — human-scannable for configuration, machine-parseable for runtime decisions.

The practical upshot is that agent documentation demands a different writing discipline. Every section must be self-contained (because retrieval systems may chunk it). Every parameter must be explicitly typed and described (because agents don't infer conventions). Every limitation must be stated (because agents won't discover them gracefully). And everything must be as concise as possible, because context windows are finite and every token competes for attention.

---

## The anatomy of a tool description that actually works

Anthropic's documentation states it plainly: **"Provide extremely detailed descriptions. This is by far the most important factor in tool performance."** They recommend at least 3–4 sentences per tool description, more for complex tools. OpenAI, Google, and every major agent framework echo this finding — description quality dominates schema structure, parameter design, and even model choice in determining whether an agent uses a tool correctly.

The most effective description formula, distilled from Composio's analysis of OpenAI, Google, and Anthropic patterns, follows a two-part structure:

```
Tool to <what it does>.
Use when <specific situation to invoke tool>.
```

This clarifies both **action** and **context**, reducing invocation errors significantly. But production-grade descriptions go further. Anthropic's engineering team recommends treating tool descriptions like onboarding documentation for a new hire: "Consider the context that you might implicitly bring — specialized query formats, definitions of niche terminology, relationships between underlying resources — and make it explicit."

Here's what a complete, effective description covers:

**A weak description:**
```json
{
  "name": "search",
  "description": "Search for things."
}
```

**A strong description:**
```json
{
  "name": "search_contacts",
  "description": "Search the company CRM for contact records matching a query. Use when the user needs to find a specific person, look up contact details, or verify a customer exists. Searches across name, email, company, and phone fields. Returns up to 20 results sorted by relevance, including name, email, company, and last interaction date. Does NOT return full conversation history — use get_contact_details for that. Query should be a natural language string, not a structured filter."
}
```

The strong version answers six questions the agent needs: what does it do, when should I use it, what fields does it search, what does it return, what does it *not* return, and how should I format the input. That "does NOT return" clause is especially valuable — Anthropic explicitly recommends documenting "what information the tool does not return if the tool name is unclear."

### Parameter descriptions require the same rigor

Every parameter needs its own description, format specification, and constraints. Vague parameter documentation is one of the most common failure modes in production agent systems.

**Weak parameter:**
```json
"date": { "type": "string", "description": "The date" }
```

**Strong parameter:**
```json
"start_date": {
  "type": "string",
  "description": "Start of the date range in ISO 8601 format (YYYY-MM-DD). Must be no more than 90 days in the past. Example: 2025-01-15",
  "format": "date"
}
```

Key principles for parameters, drawn from Composio's field guide and Anthropic's documentation:

- **Name parameters unambiguously.** Use `user_id` instead of `user`, `start_date` instead of `date`. Anthropic found this directly reduces hallucinated parameter values.
- **Use enums for constrained values.** Replace `"description": "Unit, e.g. Celsius or Fahrenheit"` with `"enum": ["celsius", "fahrenheit"]`. This eliminates an entire class of errors.
- **Embed tiny examples in descriptions.** Adding `"e.g., San Francisco, CA"` to a location parameter description measurably improves formatting accuracy.
- **Document hidden conditional rules.** If one parameter is required only when another has a specific value, state this in *both* parameter descriptions. Composio found that Firecrawl actions failed at high rates because the description didn't mention "If format is json, jsonOptions is required."
- **Declare formats explicitly** using JSON Schema's `format` keyword: `"date"`, `"email"`, `"uri"`, `"uuid"`.

### Return values and error handling close the loop

Agents need to know what they'll get back. Document return types with the same specificity as inputs. The MCP spec now supports an optional `outputSchema` for structured result validation:

```json
"outputSchema": {
  "type": "object",
  "properties": {
    "temperature": { "type": "number", "description": "Temperature in celsius" },
    "conditions": { "type": "string", "description": "Weather conditions description" },
    "humidity": { "type": "number", "description": "Humidity percentage" }
  },
  "required": ["temperature", "conditions", "humidity"]
}
```

For error handling, the MCP spec defines a critical distinction: tool execution errors should be reported *within* the result object (using `isError: true`), not as protocol-level errors. This ensures the agent sees the error and can reason about recovery. Docker's documentation team articulated why: "The agent, not the user, is the one calling your tool, and there's no guarantee it will pass the error message back to the user. Agents are designed to complete tasks. When something fails, they'll often try a different approach. That's why your error handling should help the agent decide what to do next, not just flag what went wrong."

Block's production approach makes this concrete — their error messages include recovery instructions:

```
"File '{}' is too large ({:.2}KB). Maximum size is 400KB.
If needed, use shell commands like 'head', 'tail', or 'sed -n' to read a subset of the file."
```

---

## MCP-specific practices for tools, resources, and server instructions

The Model Context Protocol defines three primitives — tools (model-controlled), resources (application-controlled), and prompts (user-controlled) — each requiring distinct documentation approaches.

### Tool definitions in MCP

Every MCP tool consists of a `name`, optional `title`, `description`, `inputSchema` (JSON Schema), optional `outputSchema`, and optional `annotations`. The annotations are particularly valuable for agent decision-making:

| Annotation | Default | Purpose |
|---|---|---|
| `readOnlyHint` | false | Signals tool doesn't modify state |
| `destructiveHint` | true | Signals tool may perform irreversible changes |
| `idempotentHint` | false | Signals repeated calls produce same result |
| `openWorldHint` | true | Signals tool interacts with external systems |

Block's production experience confirms these matter: they recommend keeping tools to a single risk level (read-only OR write, not both in one server) and using `readOnlyHint` consistently.

### Server instructions — the most underused MCP feature

MCP servers can inject instructions into the LLM's system prompt via the `instructions` field in the `initialize` response. In controlled evaluation, **server instructions improved GPT-5-Mini's workflow compliance from 20% to 80%** — a 60-percentage-point improvement. Yet most MCP servers ship without them.

Server instructions should document *relationships between tools*, not repeat individual tool descriptions:

```json
// ❌ Bad — duplicates tool descriptions
{ "instructions": "The search tool searches for files. The read tool reads files." }

// ✅ Good — adds cross-tool context
{ "instructions": "Use 'search' before 'read' to validate file paths. Search results expire after 10 minutes. Rate limit: 100 requests/minute across all tools." }
```

The official MCP blog identifies four categories of effective server instructions: **cross-feature relationships** (tool ordering dependencies), **operational patterns** (batch vs. individual calls), **constraints and limitations** (rate limits, file size caps), and **workflow sequences** (authentication-first patterns). Instructions should be concise, model-agnostic, and focused on factual tool behavior — never personality instructions or marketing language.

### Resources and resource templates

MCP resources use URIs as identifiers and support rich metadata including `name`, `title`, `description`, `mimeType`, `size`, and annotations for `audience` (user, assistant, or both) and `priority` (0.0–1.0). Resource templates enable parameterized access using RFC 6570 URI templates:

```json
{
  "uriTemplate": "github://repos/{owner}/{repo}/issues/{issue_number}",
  "name": "GitHub Issue",
  "description": "A specific GitHub issue. Returns title, body, status, labels, and comments.",
  "mimeType": "application/json"
}
```

The `audience` annotation is particularly important — marking a resource as `"audience": ["assistant"]` tells the client this content is meant for the LLM's reasoning, not for display to the user.

---

## Naming conventions that help agents pick the right tool every time

Tool naming has measurable impact on agent performance. Anthropic's engineering team found that "selecting between prefix- and suffix-based namespacing has non-trivial effects on tool-use evaluations" that vary by model. Consistent naming is non-negotiable.

The dominant convention across MCP, LangChain, and Anthropic's own tools is the **`verb_resource` pattern**: `create_ticket`, `search_contacts`, `list_repos`, `get_weather`. This maps cleanly to how agents reason about actions on entities.

For multi-service environments, **namespace with consistent prefixes**: `asana_search_tasks`, `jira_search_issues`, `slack_send_message`. Anthropic recommends three levels of specificity depending on your tool count:

- **By service:** `asana_search`, `jira_search` (when each service has few tools)
- **By resource:** `asana_projects_search`, `asana_users_search` (when services have many tools)
- **By domain:** `cameron_get_expenses`, `cameron_get_budget_status` (when tools span organizational boundaries)

The MCP spec constrains tool names to the regex `^[a-zA-Z0-9_-]{1,64}$` (Anthropic's API). When multiple servers expose tools with the same name, disambiguation strategies include concatenating the server name (`web1___search_web`) or using URI-based prefixes.

**Critical anti-pattern:** inconsistent naming across a tool library. Mixing `snake_case` and `camelCase`, or alternating between prefix namespacing (`slack_search`) and suffix namespacing (`search_slack`), directly reduces invocation accuracy. Pick one scheme and enforce it everywhere.

---

## Documenting edge cases and failure modes agents will actually encounter

Production agent systems fail most often not on the happy path, but on edge cases the documentation never mentioned. The most effective pattern is to treat **error messages themselves as a form of documentation** — agents see errors as observations and use them to plan their next action.

A practitioner writing about MCP server development learned this the hard way: their original tool description said "See which runtime flows use this function or method." The agent never triggered it. After rewriting to emphasize *use cases* — "Useful to detect possible breaking changes, check whether generated code fits current usages, generate tests based on runtime usage" — the agent seamlessly activated the tool without users explicitly requesting it.

Three specific edge case categories deserve explicit documentation:

**Temporal assumptions.** Agents often don't know the current date or time. Use relative durations in ISO 8601 format (`P7D` for 7 days) instead of expecting absolute timestamps, and document the expected syntax with examples in parameter descriptions. Anthropic discovered Claude was appending `2025` to web search queries, biasing results toward recent content — a failure that only surfaced through systematic evaluation.

**Precondition dependencies.** When tool B requires output from tool A, state this explicitly in both tool descriptions and server instructions. Agents won't discover ordering dependencies through trial and error efficiently. Document them like: "Requires a valid session_id from authenticate(). Call authenticate first if you haven't already in this session."

**Capacity limits.** A production MCP server author dumped 360K characters of JSON from an API and overwhelmed Claude 3.7 Sonnet despite its 200K token window — the model claimed no errors existed in the data. The solution: nested data hierarchies that keep initial responses focused on high-level summaries, with separate tools for drilling into details.

---

## Few-shot examples are not optional — they're transformative

LangChain ran controlled experiments on few-shot prompting for tool calling and found results that should change how everyone writes documentation. **Claude 3 Haiku jumped from 11% to 75% accuracy** with just three message-format examples. Across all models tested, any few-shotting helped significantly, and three semantically similar examples often performed as well as thirteen static ones.

Anthropic now supports an `input_examples` field in tool definitions specifically for this purpose:

```json
{
  "name": "get_weather",
  "description": "Get the current weather in a given location",
  "input_schema": {
    "type": "object",
    "properties": {
      "location": { "type": "string", "description": "City and state, e.g. San Francisco, CA" },
      "unit": { "type": "string", "enum": ["celsius", "fahrenheit"] }
    },
    "required": ["location"]
  },
  "input_examples": [
    { "location": "San Francisco, CA", "unit": "fahrenheit" },
    { "location": "Tokyo, Japan", "unit": "celsius" },
    { "location": "New York, NY" }
  ]
}
```

The third example is subtly important — it shows that `unit` is optional by omitting it. Anthropic's documentation notes: "This helps Claude understand when to include optional parameters, what formats to use, and how to structure complex inputs."

When should you include examples? **Always** for non-obvious parameter formats (dates, query syntax, IDs). **Always** when tools behave differently than training data would suggest (LangChain found agents will ignore tool output and rely on training data unless shown correction examples). **Always** for tool composition patterns — Microsoft Learn found that describing "search then fetch" workflows in descriptions improved grounding and citation quality.

One caveat from OpenAI: for reasoning models (o3/o4-mini), "adding examples may hurt performance." Test before committing to few-shot patterns with these models.

---

## Context windows are finite — every token must earn its place

Anthropic's context engineering guide states the foundational principle: "Good context engineering means finding the smallest possible set of high-signal tokens that maximize the likelihood of some desired outcome." In practice, tool definitions alone can consume enormous token budgets. Anthropic found that a five-server setup with 58 tools consumed approximately **55,000 tokens before the conversation even started**.

### Progressive disclosure has emerged as the defining solution

Rather than loading everything upfront, progressive disclosure reveals information in layers based on relevance. A three-layer implementation achieves near-perfect token efficiency:

1. **Index layer (~800 tokens):** Show tool names and one-line descriptions at session start
2. **Selection:** Agent decides what's relevant based on these summaries
3. **Detail fetch:** Retrieve full descriptions only for tools the agent actually needs

Anthropic's own Tool Search Tool implements this pattern — the agent queries a registry of available tools rather than loading all definitions. Results were dramatic: token usage dropped by **85%** while accuracy improved (Opus 4: 49% → 74%).

### Practical token-saving strategies from production

Block's engineering team (60+ MCP servers) recommends: prefer Markdown or XML over JSON for tool responses (more token-efficient), check byte sizes before returning results, truncate with clear notes rather than silently cutting off, and avoid dynamically generated long names that must be regenerated on every turn.

Phil Schmid's widely cited MCP guidance advocates **5–15 tools per server** with "one server, one job." Token cost grows with every tool description in the context window, so splitting concerns across servers lets agents load only the capability they need.

For documentation platforms, the `llms.txt` standard (proposed by Jeremy Howard, adopted by thousands of sites via Mintlify) offers a clean solution: a Markdown file at `/llms.txt` that strips away all presentational markup, leaving structured text optimized for token budgets. Fern auto-generates these from OpenAPI specs and supports query parameters like `?lang=python` to filter content to only what's relevant.

---

## JSON Schema practices that reduce agent errors

Across OpenAI, Anthropic, and Google, a consistent set of JSON Schema patterns have proven effective for agent tool definitions.

**Use `strict: true` in production.** Both OpenAI and Anthropic support constrained decoding that guarantees schema compliance when `strict: true` and `additionalProperties: false` are set. This eliminates malformed tool calls entirely, at the cost of slightly more rigid schemas (no optional properties without default handling).

**Keep schemas shallow.** Azure OpenAI caps at 100 object properties and five nesting levels. Google's Gemini documentation warns: "The API may reject very large or deeply nested schemas." Even within those limits, shallower schemas produce more reliable outputs. Phil Schmid's MCP guidance recommends flattening arguments to top-level primitives wherever possible.

**Generate schemas from type definitions, not by hand.** Use Pydantic (Python) or Zod (TypeScript) to generate JSON Schema from typed code. This prevents schema/code drift — a subtle bug where the schema you document diverges from the schema your code expects. Both OpenAI and Anthropic explicitly recommend this pattern.

**Add `_comment` fields to JSON responses.** Since JSON doesn't support comments, agents can't interpret opaque response fields. A practitioner building MCP servers found that adding a `_comment` element at the beginning of JSON responses explaining the data structure enabled agents to both explain data to users and reason about values correctly.

---

## Ten documentation mistakes that silently degrade agent performance

The research reveals a consistent set of anti-patterns that practitioners discover through painful iteration:

1. **Wrapping every API endpoint as a separate tool.** Anthropic warns: "A common error is tools that merely wrap existing software functionality." Build `schedule_event` instead of exposing `list_users`, `list_events`, and `create_event` separately. Consolidate multi-step workflows into outcome-oriented tools.

2. **Returning everything and letting the agent sort it out.** A `list_contacts` tool that returns all contacts forces the agent to read thousands of tokens. Build `search_contacts` with filtering instead. Anthropic recommends implementing pagination, range selection, and truncation with sensible defaults.

3. **Vague descriptions under 20 words.** "Manages user data" tells the agent nothing useful. Every description should answer: what does it do, when should I use it, what does it return, and what does it NOT do.

4. **Missing parameter format specifications.** A `"date"` parameter without format guidance produces dates in every format imaginable. Always specify format, provide examples, and use enums where possible.

5. **Inconsistent naming conventions.** Mixing `snake_case` and `camelCase`, or alternating prefix and suffix namespacing, directly reduces tool selection accuracy across all tested models.

6. **Not documenting authentication requirements.** "Agents won't guess that a 401 means 'authenticate first'" — state authentication prerequisites explicitly in tool descriptions and server instructions.

7. **Returning opaque technical identifiers.** Anthropic found that "merely resolving arbitrary alphanumeric UUIDs to more semantically meaningful language significantly improves Claude's precision in retrieval tasks by reducing hallucinations." Return `name` and `file_type` over `uuid` and `mime_type`.

8. **Omitting error schemas.** Stytch found that `400 Bad Request` without detail causes infinite retry loops, while `"missing required field 'customer_id'"` enables one-iteration self-correction. Fern calls error schemas "some of the highest-value content for AI-assisted code generation."

9. **Loading all tools into context simultaneously.** With 58 tools consuming ~55K tokens, accuracy drops. Use progressive disclosure, tool search tools, or split tools across focused servers.

10. **Never testing descriptions against actual agent behavior.** Microsoft Learn built automated evaluation tools and found that "small wording changes swung tool activation rates materially." Without measurement, you're optimizing blind.

---

## Testing whether your documentation actually helps agents perform

The most authoritative framework for evaluating agent documentation comes from Anthropic's engineering team, which uses a systematic loop: generate realistic tasks, pair each with verifiable outcomes, run programmatically, and measure multiple metrics beyond simple accuracy.

### Build evaluations that mirror real usage

Weak evaluation tasks produce misleading results. Anthropic contrasts:

- ❌ **Weak:** "Schedule a meeting with jane@acme.corp next week"
- ✅ **Strong:** "Schedule a meeting with Jane next week to discuss our latest Acme Corp project. Attach the notes from our last project planning meeting and reserve a conference room."

Strong tasks require multiple tool calls, ambiguity resolution, and mirror actual user workflows. They test whether your descriptions enable the agent to navigate complex, realistic scenarios.

### Track the right metrics

Beyond simple task completion, three metrics reveal documentation quality issues:

- **Redundant tool calls** suggest descriptions need better scoping — the agent is searching for the right tool by trial and error.
- **Invalid parameter errors** suggest schemas need clearer descriptions, better examples, or tighter enums.
- **Tool selection errors** (wrong tool chosen) suggest overlapping descriptions that don't sufficiently differentiate tools.

Paragon's quantitative study tested 50 cases across four axes and found that **tool description quality was the hardest lever to improve but had the most significant impact** on performance. Their framework measures tool correctness (exact match on tool selection) and task completion (LLM-judged).

### Use agents to optimize their own documentation

Anthropic's most striking finding: Claude Code, given evaluation transcripts showing where tools failed, can analyze patterns, identify description weaknesses, and generate improved descriptions that **outperform expert human-written versions** on held-out test sets. The process is simple — concatenate failure transcripts, ask the agent to identify issues and propose fixes, then validate against your evaluation suite.

Test cases should span four categories: **happy-path cases** (normal functionality), **edge cases** (boundary conditions), **adversarial cases** (attempts to break the agent), and **off-topic cases** (requests the agent should decline). An agent with twelve tools needs test cases exercising each tool individually, in combination with others, and with malformed inputs.

---

## Emerging standards are converging on shared principles

Three documentation standards have gained significant traction and reflect where the ecosystem is heading.

**`llms.txt`** (Jeremy Howard, September 2024) standardizes LLM-friendly documentation at the web-root level. Adopted by thousands of sites including Anthropic, Cloudflare, and Vercel. Provides stripped-down Markdown optimized for token budgets, with `llms-full.txt` variant for complete content.

**`AGENTS.md`** (OpenAI, Google, Cursor, and others, 2025) gives coding agents project-specific instructions — build commands, test procedures, architecture constraints. Over 20,000 GitHub repositories now include one. Key design principle: **150 lines maximum** — longer files bury critical signal.

**Anthropic Agent Skills** (December 2025) implement progressive disclosure natively: only a skill's name and description (~100 tokens) load at startup, with full instructions (<5K tokens) loading only when triggered. This pattern enables effectively unbounded capability documentation within finite context windows.

All three share a common philosophy: documentation for agents must be structured, concise, self-contained, and designed for selective retrieval rather than exhaustive consumption. The Agentic AI Foundation (AAIF), now under the Linux Foundation, is working to formalize these patterns alongside MCP and A2A protocols.

---

## Conclusion: documentation is the interface contract with intelligence

The single most important takeaway from this research is that **tool descriptions are not metadata — they are the primary interface** between your system and an intelligent agent. Small wording changes produce large performance swings. Production teams that measure and iterate on descriptions consistently outperform those that write them once and forget.

Three principles should guide every documentation decision. First, **write for a capable but literal new hire** — make implicit knowledge explicit, state constraints directly, and never assume the reader shares your context. Second, **optimize for token efficiency** — use progressive disclosure, keep tool counts per server between 5 and 15, and ensure every token in a description earns its place. Third, **close the feedback loop** — build evaluations, track tool selection and parameter errors, and use agents themselves to identify and fix documentation weaknesses.

The documentation practices that make agents effective turn out to be the same ones that make APIs genuinely developer-friendly. As Stytch observed: "An LLM ruthlessly exposes and logs any gaps." If your agent can't figure out how your tools work, neither can your users.