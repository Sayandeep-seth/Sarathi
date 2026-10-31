"""HTTP routes for the AI service."""

from fastapi import APIRouter, Depends

from app.api.security import get_settings, require_service_token
from app.models.registry import list_detectors
from app.schemas.common import HealthResponse, ModelsStatusResponse
from app.settings import Settings

public = APIRouter()
internal = APIRouter(prefix="/internal/v1", dependencies=[Depends(require_service_token)])


@public.get("/health", response_model=HealthResponse)
def health(settings: Settings = Depends(get_settings)) -> HealthResponse:
    return HealthResponse(status="ok", service="sarathi-ai", version=settings.version)


@internal.get("/models/status", response_model=ModelsStatusResponse)
def models_status() -> ModelsStatusResponse:
    return ModelsStatusResponse(detectors=list_detectors())
