FROM ghcr.io/astral-sh/uv:0.9.16 AS uv
FROM python:3.12-slim AS base
LABEL io.syntelle.project="syntelle"
COPY --from=uv /uv /uvx /bin/
WORKDIR /app
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev
COPY app ./app

FROM base AS test
RUN uv sync --frozen
COPY tests ./tests
CMD ["uv", "run", "--frozen", "pytest", "-q"]

FROM base AS runtime
RUN useradd --uid 10001 --no-create-home syntelle
USER syntelle
ENV PATH="/app/.venv/bin:$PATH"
EXPOSE 8000
CMD ["fastapi", "run", "--host", "0.0.0.0", "--port", "8000"]
