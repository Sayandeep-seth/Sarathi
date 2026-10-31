"""SARATHI AI service entrypoint (advisory analytics only)."""

from fastapi import FastAPI

from app.api.routes import internal, public


def create_app() -> FastAPI:
    app = FastAPI(
        title="SARATHI AI Service",
        description="Advisory analytics. Never authoritative for identity, payments or quota.",
        docs_url=None,  # no interactive docs/openapi on a service-to-service API
        redoc_url=None,
        openapi_url=None,
    )
    app.include_router(public)
    app.include_router(internal)
    return app


app = create_app()
