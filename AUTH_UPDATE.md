# Authentication & Deployment Documentation Plan

## Goal
Keep authentication simple with session cookies and clearly document the same-domain requirement for developers. Fix any remaining security issues.

## Changes

### 1. **Create Deployment Documentation** (new file: `docs/DEPLOYMENT.md`)

Document the **required** architecture with clear examples:

```markdown
# Deployment Guide

## Required Architecture: Same-Domain Deployment

⚠️ **IMPORTANT**: The backend API and frontend MUST be deployed on subdomains of the same domain.

### Why This is Required

This application uses session cookies with `SameSite=Strict` for security:
- Prevents CSRF attacks
- Protects against session hijacking
- Ensures cookies are only sent to your domain

### Correct Setup ✅

```
Frontend:  https://app.example.com
Backend:   https://api.example.com
```

Both on `example.com` domain - **cookies work automatically**

### Incorrect Setup ❌

```
Frontend:  https://app.example.com
Backend:   https://your-service.run.app
```

Different domains - **authentication will NOT work**

## Platform Setup Examples

### Google Cloud Run
1. Deploy backend to Cloud Run
2. Map custom domain: `api.example.com` → Cloud Run service
3. Deploy frontend to your hosting with domain: `app.example.com`
4. Configure environment:
   ```bash
   CORS_ORIGINS=https://app.example.com
   JUNJO_ENV=production
   ```

### AWS
1. Backend on ECS/App Runner with custom domain `api.example.com`
2. Frontend on Amplify/S3+CloudFront with domain `app.example.com`
3. Same CORS configuration

### Docker Compose (Development)
Frontend and backend on `localhost` - works out of the box!

## Environment Configuration

```bash
# .env for production
JUNJO_ENV=production                      # Enables https_only on cookies
CORS_ORIGINS=https://app.example.com     # Must match frontend domain
JUNJO_SESSION_SECRET=<your-secret>
JUNJO_SECURE_COOKIE_KEY=<your-key>
```

## Security Features

- ✅ Session cookies encrypted (AES-256) + signed (HMAC)
- ✅ `SameSite=Strict` prevents CSRF
- ✅ `HttpOnly` prevents XSS cookie theft
- ✅ `Secure` flag in production (HTTPS only)
- ✅ 30-day session lifetime

## Machine-to-Machine Authentication

For OTLP ingestion clients (OpenTelemetry SDKs), use **API Keys** instead of session cookies:
1. Generate API key via web UI
2. Configure SDK with API key
3. API keys work across any domain (no SameSite restriction)
```

### 2. **Update README.md**

Add prominent deployment requirement:

```markdown
## Deployment Requirements

⚠️ **The backend API and frontend MUST be on subdomains of the same domain** (e.g., `api.example.com` and `app.example.com`).

See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for detailed setup instructions.
```

### 3. **Add Startup Validation** (`app/main.py`)

Validate configuration and log clear errors:

```python
@app.on_event("startup")
async def validate_configuration():
    """Validate deployment configuration on startup."""

    # Check production settings
    if settings.junjo_env == "production":
        # Warn if CORS origins not configured
        if not settings.cors_origins or settings.cors_origins == ["*"]:
            logger.error(
                "❌ PRODUCTION ERROR: CORS_ORIGINS must be explicitly configured. "
                "Set CORS_ORIGINS to your frontend domain(s). "
                "See docs/DEPLOYMENT.md"
            )
            raise ValueError("CORS_ORIGINS required in production")

        # Log security status
        logger.info("✅ Production mode: HTTPS-only cookies enabled")
        logger.info(f"✅ CORS origins: {settings.cors_origins}")
    else:
        logger.info("⚠️  Development mode: HTTPS-only cookies disabled")

    logger.info("✅ Session cookies: Encrypted (AES-256) + Signed (HMAC)")
    logger.info("✅ CSRF protection: SameSite=Strict")
```

### 4. **Clean Up Deprecated Code**

Remove the deprecated `CurrentUserEmail` alias in `app/features/auth/dependencies.py`:

```python
# BEFORE (lines 97-99):
# Deprecated alias for backward compatibility during migration
# TODO: Remove after all code updated to use CurrentUser
CurrentUserEmail = CurrentUser

# AFTER (remove these lines):
# All code now uses CurrentUser consistently
```

Verify no code still uses `CurrentUserEmail`:
- Search and replace any remaining usage with `CurrentUser`
- Should already be done from the security implementation

### 5. **Update .env.example**

Simplify to show only the supported configuration:

```bash
# =============================================================================
# DEPLOYMENT CONFIGURATION
# =============================================================================
# IMPORTANT: Backend and frontend MUST be on same domain (e.g., api.example.com + app.example.com)
# See docs/DEPLOYMENT.md for details

# Environment (development or production)
JUNJO_ENV=development

# CORS Origins (comma-separated frontend URLs)
# Example: CORS_ORIGINS=https://app.example.com,https://admin.example.com
CORS_ORIGINS=http://localhost:5151

# Session Security (generate with: openssl rand -base64 32)
JUNJO_SESSION_SECRET=your-session-secret-here
JUNJO_SECURE_COOKIE_KEY=your-secure-cookie-key-here

# Session Configuration
# Note: Cookies use SameSite=Strict for security
# This requires same-domain deployment (see docs/DEPLOYMENT.md)
```

### 6. **Fix Security Issues** (if not already done)

From previous research, ensure these are fixed:

**a) Session Cookie Tampering Test**
- Verify `test_session_cookie_tampering` passes (was skipped before)
- Ensure SecureCookiesMiddleware is properly configured

**b) Add Configuration Validation**
- Require secrets in production
- Validate CORS origins format
- Error if trying to use wildcard in production

## Files to Create
- `docs/DEPLOYMENT.md` - Comprehensive deployment guide

## Files to Modify
- `README.md` - Add deployment requirement callout
- `backend/app/main.py` - Add startup validation
- `backend/app/features/auth/dependencies.py` - Remove deprecated `CurrentUserEmail` alias
- `backend/.env.example` - Simplify, remove cross-domain options, add clear comments

## Files to Check (No Changes)
- Session middleware config already correct (`same_site="strict"`)
- CORS middleware already configured
- Security tests should pass (86 passed, 1 skipped)

## Benefits
- ✅ Clear expectations for developers
- ✅ Single supported deployment pattern (simpler)
- ✅ Security by default (can't misconfigure)
- ✅ Helpful error messages if misconfigured
- ✅ Platform-specific examples (Cloud Run, AWS, etc.)
- ✅ No breaking changes to existing code
- ✅ Removes deprecated code

## What We're NOT Doing
- ❌ Not adding cross-domain support
- ❌ Not adding JWT
- ❌ Not adding asgi-csrf (SameSite=Strict is sufficient)
- ❌ Not adding complex configuration options

## Testing
- Verify all existing tests still pass
- Test that production startup validation catches misconfiguration
- Ensure deprecated `CurrentUserEmail` removal doesn't break anything

## Summary

**Decision**: Keep session cookies for authentication. Document same-domain requirement clearly.

**Rationale**:
- Web browser clients are the primary use case
- Machine clients already use API keys (correct approach)
- Session cookies are simpler and more secure for this architecture
- No need for JWT complexity without mobile apps or third-party integrations
- Same-domain requirement is standard web security practice
