# Security Implementation Plan

**Project**: Junjo Server - Backend Security Hardening
**Date**: November 2025
**Status**: In Progress

---

## Executive Summary

This plan addresses 3 critical security issues discovered through comprehensive security testing and implements a defense-in-depth strategy using authenticated user context and audit logging throughout all application layers.

### Security Issues Identified

1. **Session Cookie Tampering** (CRITICAL) - Tampered session cookies not being rejected
2. **First User Race Condition** (HIGH) - Multiple concurrent "first user" creations succeed
3. **Unprotected API Endpoints** (CRITICAL) - API keys endpoints accessible without authentication

### Defense-in-Depth Strategy

Implement comprehensive audit logging by passing `AuthenticatedUser` objects through all layers (router → service → repository), enabling full visibility into who performed which actions while maintaining clean separation of concerns.

**Key Principle**: API keys are **shared resources** (any authenticated user can CRUD), but all actions must be auditable.

---

## Phase 1: Investigation & Research Results

### 1.1 Library Versions Status ✅

| Library | Required | Latest | Status | Notes |
|---------|----------|--------|--------|-------|
| starlette-securecookies | >=1.1.1 | 1.1.1 | ✅ Current | Inactive maintenance (12+ months) |
| itsdangerous | >=2.2.0 | 2.2.0 | ✅ Current | No vulnerabilities, widely used |
| asgi-csrf | >=0.11 | 0.11 | ✅ Current | Already in dependencies but not used |
| fastapi | >=0.115.0 | 0.115+ | ✅ Current | - |
| starlette | (via fastapi) | 0.47+ | ✅ Current | SessionMiddleware included |

**Conclusion**: All libraries are at latest stable versions. No security updates available.

### 1.2 ASGI-CSRF Investigation Results

**Purpose**: CSRF protection via Double Submit Cookie pattern
**Mechanism**: Sets `csrftoken` cookie that must match token in form data or headers

**Integration Pattern**:
```python
from asgi_csrf import asgi_csrf
app = asgi_csrf(app, signing_secret="secret")
```

**Findings**:
- ✅ Provides additional CSRF protection layer
- ✅ Complements existing SessionMiddleware `same_site="strict"`
- ❌ Does NOT prevent session cookie tampering
- ❌ Does NOT validate session integrity
- ⚠️  May be redundant given our `same_site="strict"` setting

**Recommendation**: **Not needed at this time**. Our SessionMiddleware with `same_site="strict"` already provides CSRF protection. Re-evaluate if we need cross-origin state-changing requests.

### 1.3 Session Cookie Tampering Root Cause Analysis

**Current Middleware Stack** (`app/main.py:140-153`):
```python
# Layer 1: Encryption (outer)
app.add_middleware(
    SecureCookiesMiddleware,
    secrets=[settings.session_cookie.secure_cookie_key],  # 32-byte Fernet key
)

# Layer 2: Signing (inner)
app.add_middleware(
    SessionMiddleware,
    secret_key=settings.session_cookie.session_secret,  # HMAC signing
    max_age=86400 * 30,
    https_only=True,
    same_site="strict",
)
```

**Expected Flow**:
1. **Outgoing**: SessionMiddleware signs → SecureCookiesMiddleware encrypts
2. **Incoming**: SecureCookiesMiddleware decrypts → SessionMiddleware verifies signature

**Test Failure**: `test_session_cookie_tampering` shows tampered cookies ARE being accepted

**Hypotheses for Investigation**:
1. ❓ SecureCookiesMiddleware may not validate HMAC on decryption
2. ❓ Test environment secrets may differ from runtime secrets
3. ❓ Middleware order may cause signature validation to be skipped
4. ❓ AsyncClient test client may bypass middleware layers

**Next Steps**:
- Create middleware unit tests in isolation
- Add debug logging to trace cookie validation
- Test with real HTTP requests (not test client)
- Consider starlette-securecookies inactive maintenance as risk factor

---

## Implementation Phases

### Phase 2: Create AuthenticatedUser Model ⏳

**Objective**: Create immutable context object to flow through all layers

**Files to Create**:
- `backend/app/features/auth/models.py`
- `backend/app/common/audit.py`

**Files to Modify**:
- `backend/app/features/auth/dependencies.py` - Update to return AuthenticatedUser
- `backend/app/features/auth/router.py` - Store session metadata on sign-in

**AuthenticatedUser Structure**:
```python
@dataclass(frozen=True)
class AuthenticatedUser:
    email: str
    user_id: str
    authenticated_at: datetime
    session_id: str
```

### Phase 3: Audit Logging Infrastructure ⏳

**Objective**: Structured logging with user context at every layer

**Pattern**:
```python
audit_log("create", "api_key", key_id, authenticated_user, {"name": name})
```

**Output Format**:
```
AUDIT: CREATE api_key | user_email=user@example.com user_id=usr_123
session_id=abc123... resource_id=key_456 details={"name": "Production Key"}
```

### Phase 4-5: Update Features with AuthenticatedUser ⏳

**Apply to**:
- API Keys (Phase 4)
- Users (Phase 5)
- LLM Playground (Phase 8)

**Pattern for Each Feature**:
1. Router: Add `authenticated_user: CurrentUser` dependency
2. Service: Add `authenticated_user` parameter + audit logging
3. Repository: Add `authenticated_user` parameter + data-layer audit logging

**No Ownership Filtering**: API keys are shared - any authenticated user can CRUD

### Phase 6: Fix First User Race Condition ⏳

**Solution**: Transaction with SELECT FOR UPDATE

```python
async with session.begin():
    # Lock users table
    stmt = select(func.count()).select_from(UserTable).with_for_update()
    count = (await session.execute(stmt)).scalar_one()

    if count > 0:
        raise ValueError("Users already exist")

    # Create user (lock held until commit)
    db_obj = UserTable(email=email, password_hash=password_hash)
    session.add(db_obj)
```

### Phase 7: Fix Session Cookie Tampering ⏳

**Actions**:
1. Create middleware unit tests (`test_session_middleware.py`)
2. Add debug logging to trace cookie validation
3. Investigate starlette-securecookies source for HMAC validation
4. Consider alternative: server-side sessions if client-side unsolvable

**Test to Pass**: `test_session_cookie_tampering`

### Phase 9: Update All Tests ⏳

**Add Fixture**:
```python
@pytest.fixture
def mock_authenticated_user() -> AuthenticatedUser:
    return AuthenticatedUser(
        email="test@example.com",
        user_id="test-user-id",
        authenticated_at=datetime.now(),
        session_id="test-session-id"
    )
```

**Update All Tests** to pass `authenticated_user` parameter

### Phase 10: Update AGENTS.md Documentation ⏳

**Remove**: Go backend examples (deleted code)

**Add**:
- Python backend defense-in-depth pattern
- CRUD examples with AuthenticatedUser
- Audit logging patterns
- Security testing guidelines

---

## Testing Strategy

### Security Tests to Pass

1. `test_session_cookie_tampering` - Currently SKIPPED (needs fix)
2. `test_create_first_user_race_condition` - Currently SKIPPED (needs fix)
3. `test_protected_endpoints_require_auth` - Update expectations for API keys

### Test Execution

```bash
# Run security tests
uv run python -m pytest tests/security/ -v

# Run specific feature tests
uv run python -m pytest tests/test_api_keys_* -v

# Run all tests
uv run python -m pytest tests/ -v
```

### Success Criteria

- ✅ All 3 security issues fixed
- ✅ 0 skipped security tests
- ✅ 85+ tests passing
- ✅ Audit logs capture all CRUD operations with user context
- ✅ No breaking changes to existing functionality

---

## Risk Mitigation

### Potential Issues

1. **Breaking Change**: API keys endpoints will require authentication
   - **Impact**: Any existing clients calling these endpoints will break
   - **Mitigation**: These endpoints were never intended to be public - this is a bug fix

2. **Performance**: Additional database query per request for AuthenticatedUser
   - **Impact**: +1 query to fetch user details in dependency
   - **Mitigation**: Consider caching user in session if becomes bottleneck

3. **Test Updates**: All tests need authenticated user parameter
   - **Impact**: Large refactor across test suite
   - **Mitigation**: Create shared fixtures, update incrementally

### Rollback Plan

- Git history preserves all original code
- No database migrations required (only additive changes)
- Feature can be deployed incrementally (feature by feature)

---

## Timeline

| Phase | Estimated Time | Status |
|-------|---------------|--------|
| Phase 1: Investigation | 2 hours | ✅ Complete |
| Phase 2: AuthenticatedUser Model | 1 hour | ⏳ Pending |
| Phase 3: Audit Logging | 1 hour | ⏳ Pending |
| Phase 4: API Keys Update | 2 hours | ⏳ Pending |
| Phase 5: Users Update | 1 hour | ⏳ Pending |
| Phase 6: Race Condition Fix | 1 hour | ⏳ Pending |
| Phase 7: Cookie Tampering Fix | 3 hours | ⏳ Pending |
| Phase 8: LLM Update | 1 hour | ⏳ Pending |
| Phase 9: Test Updates | 3 hours | ⏳ Pending |
| Phase 10: Documentation | 2 hours | ⏳ Pending |
| **Total** | **17 hours** | **6% Complete** |

---

## Appendix: Key Design Decisions

### Why Not Ownership Filtering?

**User Requirement**: "API Keys are not owned by users. Any user should be able to CRUD them."

This means:
- ✅ Authentication required (prevent anonymous access)
- ✅ Audit logging (track who did what)
- ❌ No authorization filtering (no `WHERE user_email = ?`)
- ❌ No per-user isolation

### Why AuthenticatedUser vs. Just Email?

**Benefits of full object**:
1. Type safety (can't accidentally pass wrong string)
2. Rich context for audit logging (session_id, authenticated_at)
3. Immutable (frozen dataclass prevents tampering)
4. Extensible (can add roles, permissions later)
5. Self-documenting (clear what's expected)

### Why Not Use asgi-csrf?

**Reasoning**:
1. SessionMiddleware with `same_site="strict"` already provides CSRF protection
2. asgi-csrf doesn't solve our session tampering issue
3. Adds complexity without clear benefit
4. Can always add later if cross-origin requests needed

---

## References

- [asgi-csrf Documentation](https://github.com/simonw/asgi-csrf)
- [Starlette SessionMiddleware](https://www.starlette.io/middleware/#sessionmiddleware)
- [FastAPI Security Best Practices](https://fastapi.tiangolo.com/tutorial/security/)
- [OWASP Defense in Depth](https://owasp.org/www-community/Defense_in_Depth)

---

**Last Updated**: 2025-11-01
**Next Review**: After Phase 7 completion
