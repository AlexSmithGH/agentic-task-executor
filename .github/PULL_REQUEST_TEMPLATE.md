## Description

<!-- Provide a clear and concise description of what this PR does -->

## Related Issues

<!-- Link to related JIRA tickets or GitHub issues -->
- Closes: ROSAENG-XXXXX
- Related: #XXX

## Type of Change

<!-- Mark relevant options with an [x] -->

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Configuration change
- [ ] Refactoring (no functional changes)
- [ ] Performance improvement
- [ ] Security fix

## Changes Made

<!-- Provide a detailed list of changes -->

- 
- 
- 

## Testing Done

<!-- Describe the testing you performed -->

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing performed
- [ ] Tested locally with `make dev`

**Test Results:**
```
<!-- Paste test output here -->
```

## Temporal Workflow Changes

<!-- If this PR modifies workflows, confirm determinism -->

- [ ] Workflow changes maintain determinism
- [ ] No use of `time.Now()` or random values in workflows
- [ ] Activity timeouts configured appropriately
- [ ] Error handling follows project patterns

## Security Considerations

<!-- Address any security implications -->

- [ ] No credentials or secrets exposed
- [ ] Input validation added where needed
- [ ] Workspace path validation for file operations
- [ ] Command execution properly sanitized

## Documentation

- [ ] README.md updated (if needed)
- [ ] CLAUDE.md updated (if needed)
- [ ] API documentation updated (if needed)
- [ ] Architecture docs updated (if needed)
- [ ] Code comments added for complex logic

## Checklist

- [ ] My code follows the project's code style
- [ ] I have performed a self-review of my code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] My changes generate no new warnings
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes
- [ ] Any dependent changes have been merged and published

## Quality Gates

<!-- Verify all quality gates pass -->

- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make build` succeeds
- [ ] No gosec warnings introduced

## Deployment Notes

<!-- Any special deployment considerations? -->

- [ ] No special deployment steps required
- [ ] Database migration needed
- [ ] Configuration changes required
- [ ] Service restart required

**Configuration Changes:**
<!-- List any new environment variables or config changes -->

```bash
# Add to .env:
# NEW_VAR=value
```

## Screenshots/Logs

<!-- If applicable, add screenshots or logs to demonstrate changes -->

```
<!-- Paste relevant logs here -->
```

## Reviewer Guidance

<!-- Help reviewers understand what to focus on -->

**Focus Areas:**
- 
- 

**Questions:**
- 
- 

## Post-Merge Actions

<!-- Any follow-up actions needed after merge? -->

- [ ] Update staging environment
- [ ] Notify team in Slack
- [ ] Update related documentation
- [ ] Monitor for errors in production

---

**Additional Context:**
<!-- Add any other context about the PR here -->
