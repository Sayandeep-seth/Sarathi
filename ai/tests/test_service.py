import pytest
from fastapi.testclient import TestClient

from app.api.security import get_settings
from app.main import create_app
from app.settings import Settings


def client(token: str | None) -> TestClient:
    app = create_app()
    app.dependency_overrides[get_settings] = lambda: Settings(service_token=token, version="t1")
    return TestClient(app)


def test_health_is_public():
    r = client(None).get("/health")
    assert r.status_code == 200
    assert r.json() == {"status": "ok", "service": "sarathi-ai", "version": "t1"}


def test_internal_fails_closed_without_configured_token():
    r = client(None).get("/internal/v1/models/status", headers={"X-Service-Token": "anything"})
    assert r.status_code == 503


@pytest.mark.parametrize("headers", [{}, {"X-Service-Token": "wrong"}, {"X-Service-Token": ""}])
def test_internal_rejects_missing_or_wrong_token(headers):
    r = client("s3cret").get("/internal/v1/models/status", headers=headers)
    assert r.status_code == 401


def test_models_status_is_honest_and_advisory():
    r = client("s3cret").get("/internal/v1/models/status", headers={"X-Service-Token": "s3cret"})
    assert r.status_code == 200
    body = r.json()
    assert body["authority"] == "advisory_only"
    assert body["detectors"], "expected planned detectors to be listed"
    assert all(d["validated"] is False for d in body["detectors"])


def test_no_interactive_docs_exposed():
    c = client("s3cret")
    assert c.get("/docs").status_code == 404
    assert c.get("/openapi.json").status_code == 404
