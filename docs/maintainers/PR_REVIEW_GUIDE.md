# 🔍 PR Review Guide for Maintainers

**Language:** [English](PR_REVIEW_GUIDE.md) | [中文](PR_REVIEW_GUIDE.zh-CN.md)

This guide is for Alpha Trading maintainers reviewing pull requests.

---

## 📋 Review Checklist

### 1. Initial Triage (Within 24 hours)

- [ ] **Check PR alignment with roadmap**
  - Does it fit into our current priorities?
  - Is it in the [roadmap](../roadmap/README.md)?
  - If not, should we accept it anyway?

- [ ] **Verify PR completeness**
  - All sections of PR template filled?
  - Clear description of changes?
  - Related issues linked?
  - Screenshots/demo for UI changes?

- [ ] **Apply appropriate labels**
  - Priority: critical/high/medium/low
  - Type: bug/feature/enhancement/docs
  - Area: frontend/backend/exchange/ai/security
  - Status: needs review/needs changes

- [ ] **Assign reviewers**
  - Assign based on area of expertise
  - At least 1 maintainer review required

### 2. Code Review

#### A. Functionality Review

```markdown
✅ **Questions to Ask:**

- Does it solve the stated problem?
- Are edge cases handled?
- Will this break existing functionality?
- Is the approach correct for our architecture?
- Are there better alternatives?
```

**Testing:**
- [ ] All CI checks passed?
- [ ] Manual testing performed by contributor?
- [ ] Test coverage adequate?
- [ ] Tests are meaningful (not just for coverage)?

#### B. Code Quality Review

**Go Backend Code:**

```go
// ❌ Bad - Reject
func GetData(a, b string) interface{} {
    d := doSomething(a, b)
    return d
}

// ✅ Good - Approve
func GetAccountBalance(apiKey, secretKey string) (*Balance, error) {
    if apiKey == "" || secretKey == "" {
        return nil, fmt.Errorf("API credentials required")
    }

    balance, err := client.FetchBalance(apiKey, secretKey)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch balance: %w", err)
    }

    return balance, nil
}
```

**Check for:**
- [ ] Meaningful variable/function names
- [ ] Proper error handling (no ignored errors)
- [ ] Comments for complex logic
- [ ] No hardcoded values (use constants/config)
- [ ] Follows Go idioms and conventions
- [ ] No unnecessary complexity

**TypeScript/React Frontend Code:**

```typescript
// ❌ Bad - Reject
const getData = (data: any) => {
  return data.map(d => <div>{d.name}</div>)
}

// ✅ Good - Approve
interface Trader {
  id: string;
  name: string;
  status: 'running' | 'stopped';
}

const TraderList: React.FC<{ traders: Trader[] }> = ({ traders }) => {
  return (
    <div className="trader-list">
      {traders.map(trader => (
        <TraderCard key={trader.id} trader={trader} />
      ))}
    </div>
  );
};
```

**Check for:**
- [ ] Type safety (no `any` unless absolutely necessary)
- [ ] Proper React patterns (hooks, functional components)
- [ ] Component reusability
- [ ] Accessibility (a11y) considerations
- [ ] Performance optimizations (memoization where needed)

#### C. Security Review

**Critical Checks:**

```go
// 🚨 REJECT - Security Issue
func Login(username, password string) {
    query := "SELECT * FROM users WHERE username='" + username + "'"  // SQL Injection!
    db.Query(query)
}

// ✅ APPROVE - Secure
func Login(username, password string) error {
    query := "SELECT * FROM users WHERE username = ?"
    row := db.QueryRow(query, username)  // Parameterized query
    // ... proper password verification with bcrypt
}
```

- [ ] No SQL injection vulnerabilities
- [ ] No XSS vulnerabilities in frontend
- [ ] API keys/secrets not hardcoded
- [ ] User inputs properly validated
- [ ] Authentication/authorization properly handled
- [ ] No sensitive data in logs
- [ ] Dependencies have no known vulnerabilities

#### D. Performance Review

- [ ] No obvious performance issues
- [ ] Database queries optimized (indexes, no N+1 queries)
- [ ] No unnecessary API calls
- [ ] Proper caching where applicable
- [ ] No memory leaks

### 3. Documentation Review

- [ ] Code comments for complex logic
- [ ] README updated if needed
- [ ] API documentation updated (if API changes)
- [ ] Migration guide for breaking changes
- [ ] Changelog entry (for significant changes)

### 4. Testing Review

- [ ] Unit tests for new functions
- [ ] Integration tests for new features
- [ ] Tests actually test the functionality (not just coverage)
- [ ] Test names are descriptive
- [ ] Mock data is realistic

---

## 🏷️ Label Management

### Priority Assignment

Use these criteria to assign priority:

**Critical:**
- Security vulnerabilities
- Production-breaking bugs
- Data loss issues

**High:**
- Major bugs affecting many users
- High-priority roadmap features
- Performance issues

**Medium:**
- Regular bug fixes
- Standard feature requests
- Refactoring

**Low:**
- Minor improvements
- Code style changes
- Non-urgent documentation

### Status Workflow

```
needs review → in review → needs changes → needs review → approved → merged
                       ↓
                   on hold
```

**Status Labels:**
- `status: needs review` - Ready for initial review
- `status: in progress` - Being actively reviewed
- `status: needs changes` - Reviewer requested changes
- `status: on hold` - Waiting for discussion/decision
- `status: blocked` - Blocked by another PR/issue

---

## 💬 Providing Feedback

### Writing Good Review Comments

**❌ Bad Comments:**
```
This is wrong.
Change this.
Why did you do this?
```

**✅ Good Comments:**
```
This approach might cause issues with concurrent requests.
Consider using a mutex or atomic operations here.

Suggestion: Extract this logic into a separate function for better testability:
```go
func validateTraderConfig(config *TraderConfig) error {
    // validation logic
}
```

Question: Have you considered using the existing `ExchangeClient` interface
instead of creating a new one? This would maintain consistency with the rest
of the codebase.
```

### Comment Types

**🔴 Blocking (must be addressed):**
```markdown
**BLOCKING:** This introduces a SQL injection vulnerability.
Please use parameterized queries instead.
```

**🟡 Non-blocking (suggestions):**
```markdown
**Suggestion:** Consider using `strings.Builder` here for better performance
when concatenating many strings.
```

**🟢 Praise (encourage good practices):**
```markdown
**Nice!** Great use of context for timeout handling. This is exactly what
we want to see.
```

### Questions vs Directives

**❌ Directive (can feel demanding):**
```
Change this to use the factory pattern.
Add tests for this function.
```

**✅ Question (more collaborative):**
```
Would the factory pattern be a better fit here? It might make testing easier.
Could you add a test case for the error path? I want to make sure we handle
failures gracefully.
```

---

## ⏱️ Response Time Guidelines

| PR Type | Initial Review | Follow-up | Merge Decision |
|---------|---------------|-----------|----------------|
| **Critical Bug** | 4 hours | 2 hours | Same day |
| **Bounty PR** | 24 hours | 12 hours | 2-3 days |
| **Feature** | 2-3 days | 1-2 days | 3-5 days |
| **Documentation** | 2-3 days | 1-2 days | 3-5 days |
| **Large PR** | 3-5 days | 2-3 days | 5-7 days |

---

## ✅ Approval Criteria

A PR should be approved when:

1. **Functionality**
   - ✅ Solves the stated problem
   - ✅ No regression in existing features
   - ✅ Edge cases handled

2. **Quality**
   - ✅ Follows code standards
   - ✅ Well-structured and readable
   - ✅ Adequate test coverage

3. **Security**
   - ✅ No security vulnerabilities
   - ✅ Inputs validated
   - ✅ Secrets properly managed

4. **Documentation**
   - ✅ Code commented where needed
   - ✅ Docs updated if applicable

5. **Process**
   - ✅ All CI checks pass
   - ✅ All review comments addressed
   - ✅ Rebased on latest dev branch

---

## 🚫 Rejection Criteria

Reject a PR if:

**Immediate Rejection:**
- 🔴 Introduces security vulnerabilities
- 🔴 Contains malicious code
- 🔴 Violates Code of Conduct
- 🔴 Contains plagiarized code
- 🔴 Hardcoded API keys or secrets

**Request Changes:**
- 🟡 Poor code quality (after feedback ignored)
- 🟡 No tests for new features
- 🟡 Breaking changes without migration path
- 🟡 Doesn't align with roadmap (without prior discussion)
- 🟡 Incomplete (missing critical parts)

**Close with Explanation:**
- 🟠 Duplicate functionality
- 🟠 Out of scope for project
- 🟠 Better alternative already exists
- 🟠 Contributor unresponsive for >2 weeks

---

## 🎯 Special Case Reviews

### Bounty PRs

Extra care needed:

- [ ] All acceptance criteria met?
- [ ] Demo video/screenshots provided?
- [ ] Working as specified in bounty issue?
- [ ] Payment info discussed privately?
- [ ] Priority review (24h turnaround)

### Breaking Changes

- [ ] Migration guide provided?
- [ ] Deprecation warnings added?
- [ ] Version bump planned?
- [ ] Backward compatibility considered?
- [ ] RFC (Request for Comments) created for major changes?

### Security PRs

- [ ] Verified by security-focused reviewer?
- [ ] No public disclosure of vulnerability?
- [ ] Coordinated disclosure if needed?
- [ ] Security advisory prepared?
- [ ] Patch release planned?

---

## 🔄 Merge Guidelines

### When to Merge

Merge when:
- ✅ At least 1 approval from maintainer
- ✅ All CI checks passing
- ✅ All conversations resolved
- ✅ No requested changes pending
- ✅ Rebased on latest target branch

### Merge Strategy

**Squash Merge** (default for most PRs):
- Small bug fixes
- Single-feature PRs
- Documentation updates
- Keeps git history clean

**Merge Commit** (for complex PRs):
- Multi-commit features with logical commits
- Preserve commit history
- Large refactoring with atomic commits

**Rebase and Merge** (rarely):
- When linear history is important
- Commits are already well-structured

### Merge Commit Message

Format:
```
<type>(<scope>): <PR title> (#123)

Brief description of changes.

- Key change 1
- Key change 2

Co-authored-by: Contributor Name <email@example.com>
```

---

## 📊 Review Metrics to Track

Monitor these metrics monthly:

- Average time to first review
- Average time to merge
- PR acceptance rate
- Number of PRs by type (bug/feature/docs)
- Number of PRs by area (frontend/backend/exchange)
- Contributor retention rate

---

## 🙋 Questions?

If unsure about a PR:

1. **Ask other maintainers** in private channel
2. **Request more context** from contributor
3. **Mark as "on hold"** and add to next maintainer sync
4. **When in doubt, be conservative** - better to ask than approve something risky

---

## 🔗 Related Resources

- [Contributing Guide](../../CONTRIBUTING.md)
- [Code of Conduct](../../CODE_OF_CONDUCT.md)
- [Security Policy](../../SECURITY.md)
- [Project Roadmap](../roadmap/README.md)

---

**Remember:** Reviews should be **respectful**, **constructive**, and **educational**.
We're building a community, not just code. 🚀
