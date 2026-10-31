"""Service-to-service authentication for /internal routes."""

import hmac

from fastapi import Depends, Header, HTTPException, status

from app.settings import Settings


def get_settings() -> Settings:
    return Settings()


def require_service_token(
    x_service_token: str | None = Header(default=None),
    settings: Settings = Depends(get_settings),
) -> None:
    """Fail closed: no configured token means internal routes are unavailable."""
    if not settings.service_token:
        raise HTTPException(status.HTTP_503_SERVICE_UNAVAILABLE, "service authentication not configured")
    supplied = x_service_token or ""
    if not hmac.compare_digest(supplied.encode(), settings.service_token.encode()):
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "invalid service credentials")
