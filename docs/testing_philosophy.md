# Testing Philosophy

## When to Write Tests

Something is test-worthy if it meets **at least one** of these criteria:

### A) Unlikely to Change

Stable interfaces and core algorithms that won't be refactored frequently. Tests here protect against regressions without creating maintenance burden.

**Examples:**

- Binary search functions (`SearchTbls`, `SearchCols`, `SearchFks`)
- SQL operator mappings (`mapColType`, `mapOnAction`)
- Error sentinel definitions

**Not worth testing:**

- Handler routing (changes with API design)
- Response formatting (cosmetic changes)

### B) Has Many Edge Cases

Logic with multiple code paths, boundary conditions, or input variations that are easy to miss.

**Examples:**

- WHERE clause building (12+ operators, NOT variants, AND/OR combinations)
- Schema parsing (NULL defaults, composite keys, FTS tables, views)
- Input validation (empty values, invalid types, missing fields)

**Not worth testing:**

- Simple pass-through functions
- Single code path operations

### C) Contains Complex Context

Operations where the correct behavior depends on multiple interacting pieces of state that are hard to reason about.

**Examples:**

- Foreign key relationship discovery
- Nested JOIN query building
- Schema cache invalidation

**Not worth testing:**

- Pure functions with obvious inputs/outputs
- Thin wrappers around well-tested libraries

## What NOT to Test

- **Happy path only scenarios** - If there's only one way something can work, a test adds little value
- **Implementation details** - Test behavior, not internal structure
- **Third-party code** - Trust SQLite, the standard library, etc.
- **Rapidly changing features** - Wait until the API stabilizes
- **Trivial code** - Getters, setters, simple constructors

## Test Structure

When tests are warranted, follow this structure:

1. **Constants/variables at top** - SQL schemas, test data, request fixtures
2. **Descriptive comments** - Explain what edge case each fixture tests
3. **Table-driven tests** - For functions with many input variations
4. **Helper functions** - `setupTestDB`, `parseJSONArray`, etc.
5. **Real-world testing** - Use real in-memory sqlite databases to test against

## When to Delete Tests

Tests are liabilities too. Delete a test when:

- The feature it tests was removed
- It tests implementation details that changed
- It's flaky and the fix cost exceeds the value
- It duplicates coverage from another test

## Real Database vs Mocks

Use real SQLite for tests. Reasons:

- SQLite is fast enough (in-memory or file-based)
- Mocks drift from real behavior over time
- SQL edge cases are best caught by real SQL execution
- Less test code to maintain

Only mock external services (HTTP APIs, file systems in CI, etc.)

## Test Speed

If a test takes more than 100ms, question whether it's testing too much. Slow tests get skipped by developers and lose their value.

## Rule of Thumb

Before writing a test, ask: "If this breaks, will the test catch it before a user does, and is that worth the maintenance cost?"

If the answer is no to either part, skip the test.
