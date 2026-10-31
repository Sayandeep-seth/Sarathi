"""Shared response schemas."""

from typing import Literal

from pydantic import BaseModel, ConfigDict

Severity = Literal["info", "low", "medium", "high"]


class HealthResponse(BaseModel):
    model_config = ConfigDict(frozen=True)
    status: Literal["ok"]
    service: str
    version: str


class DetectorStatus(BaseModel):
    """Describes one registered detector and, honestly, whether it has been validated."""

    name: str
    kind: Literal["statistical_rule", "ml_model"]
    version: str
    validated: bool
    notes: str


class ModelsStatusResponse(BaseModel):
    detectors: list[DetectorStatus]
    # Policy statement surfaced to callers so no one mistakes this for an authority.
    authority: Literal["advisory_only"] = "advisory_only"
