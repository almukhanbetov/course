---
name: testing-system
description: Build quizzes, tests, attempts, scoring and course completion checks.
---

# Testing System

## Entities
- tests
- questions
- answers
- test_attempts
- attempt_answers (when detailed review is needed)

## Rules
- Never send correct-answer flags to the client before submission.
- Score on the backend.
- Store attempt results.
- Limit attempts only when the product rule requires it.
- Final tests may participate in course-completion logic.

## Typical flow
1. User starts test.
2. Backend returns questions without answer keys.
3. User submits answers.
4. Backend calculates score.
5. Attempt is saved.
6. Backend returns score/result.
