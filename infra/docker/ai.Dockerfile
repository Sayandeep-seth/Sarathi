# syntax=docker/dockerfile:1
# NOTE: not built in the authoring sandbox (no Docker). Verify with `make docker-build`.
FROM python:3.12-slim AS build
WORKDIR /w
COPY ai/requirements.txt .
RUN python -m venv /venv && /venv/bin/pip install --no-cache-dir -r requirements.txt

FROM python:3.12-slim
RUN useradd --system --uid 10001 --no-create-home sarathi
COPY --from=build /venv /venv
WORKDIR /app
COPY ai/app ./app
ENV PATH=/venv/bin:$PATH PYTHONUNBUFFERED=1
USER 10001
EXPOSE 8090
HEALTHCHECK --interval=15s --timeout=3s --retries=3 \
  CMD python -c "import urllib.request;urllib.request.urlopen('http://127.0.0.1:8090/health',timeout=2)"
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8090"]
