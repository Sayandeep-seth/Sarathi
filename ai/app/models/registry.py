"""Registry of analytics detectors.

Milestone 1 registers only the *planned* rule detectors, all marked unvalidated.
Detector logic and its evaluation harness arrive in Milestone 7. Nothing here may
authorize payments, grant quota, authenticate users or alter gateway security.
"""

from app.schemas.common import DetectorStatus

_DETECTORS: tuple[DetectorStatus, ...] = (
    DetectorStatus(
        name="usage_spike_zscore",
        kind="statistical_rule",
        version="0.0.0-planned",
        validated=False,
        notes="Planned in Milestone 7; not implemented yet. Needs per-cab hourly byte totals.",
    ),
    DetectorStatus(
        name="gateway_heartbeat_gap",
        kind="statistical_rule",
        version="0.0.0-planned",
        validated=False,
        notes="Planned in Milestone 7; not implemented yet. Needs gateway heartbeat timestamps.",
    ),
)


def list_detectors() -> list[DetectorStatus]:
    return list(_DETECTORS)
