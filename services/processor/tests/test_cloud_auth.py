from fastapi.testclient import TestClient
from app.main import app
def test_cloud_requires_service_secret(monkeypatch):
    monkeypatch.setenv("SYNTELLE_DEPLOYMENT_MODE", "cloud")
    monkeypatch.setenv("SYNTELLE_PROCESSOR_TOKEN", "s" * 32)
    with TestClient(app) as client:
        assert client.get("/healthz").status_code == 200
        assert client.post("/internal/v1/images/validate").status_code == 403
        assert client.post("/internal/v1/images/validate", headers={"Authorization": "Bearer " + "s" * 32}).status_code == 400
