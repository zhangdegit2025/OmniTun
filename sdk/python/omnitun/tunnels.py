from __future__ import annotations

from typing import TYPE_CHECKING, Any, Dict, List, Optional

if TYPE_CHECKING:
    from .client import OmniTun, Tunnel


class TunnelsAPI:
    def __init__(self, client: OmniTun):
        self._client = client

    def create(
        self,
        name: str,
        protocol: str,
        local_port: int,
        remote_port: Optional[int] = None,
        domain: Optional[str] = None,
        tags: Optional[List[str]] = None,
        auth_mode: Optional[str] = None,
        tls_mode: Optional[str] = None,
        compression: bool = False,
    ) -> Dict[str, Any]:
        body: Dict[str, Any] = {
            "name": name,
            "protocol": protocol,
            "local_port": local_port,
        }
        if remote_port is not None:
            body["remote_port"] = remote_port
        if domain is not None:
            body["domain"] = domain
        if tags is not None:
            body["tags"] = tags
        if auth_mode is not None:
            body["auth_mode"] = auth_mode
        if tls_mode is not None:
            body["tls_mode"] = tls_mode
        if compression:
            body["compression"] = compression

        return self._client.request("POST", "/v1/tunnels", json_body=body)

    def get(self, tunnel_id: str) -> Dict[str, Any]:
        return self._client.request("GET", f"/v1/tunnels/{tunnel_id}")

    def list(
        self,
        status: Optional[str] = None,
        protocol: Optional[str] = None,
        page: int = 1,
        per_page: int = 20,
    ) -> List[Dict[str, Any]]:
        params: Dict[str, Any] = {"page": page, "per_page": per_page}
        if status:
            params["status"] = status
        if protocol:
            params["protocol"] = protocol
        data = self._client.request("GET", "/v1/tunnels", json_body=params)
        return data.get("data", []) if isinstance(data, dict) else []

    def update(
        self,
        tunnel_id: str,
        name: Optional[str] = None,
        protocol: Optional[str] = None,
        local_port: Optional[int] = None,
        domain: Optional[str] = None,
        tags: Optional[List[str]] = None,
        auth_mode: Optional[str] = None,
        tls_mode: Optional[str] = None,
        compression: Optional[bool] = None,
    ) -> Dict[str, Any]:
        body: Dict[str, Any] = {}
        if name is not None:
            body["name"] = name
        if protocol is not None:
            body["protocol"] = protocol
        if local_port is not None:
            body["local_port"] = local_port
        if domain is not None:
            body["domain"] = domain
        if tags is not None:
            body["tags"] = tags
        if auth_mode is not None:
            body["auth_mode"] = auth_mode
        if tls_mode is not None:
            body["tls_mode"] = tls_mode
        if compression is not None:
            body["compression"] = compression

        return self._client.request("PATCH", f"/v1/tunnels/{tunnel_id}", json_body=body)

    def delete(self, tunnel_id: str) -> None:
        self._client.request("DELETE", f"/v1/tunnels/{tunnel_id}")

    def start(self, tunnel_id: str) -> Dict[str, Any]:
        return self._client.request("POST", f"/v1/tunnels/{tunnel_id}/start")

    def stop(self, tunnel_id: str) -> Dict[str, Any]:
        return self._client.request("POST", f"/v1/tunnels/{tunnel_id}/stop")

    def connect(self, tunnel_id: str):
        raise NotImplementedError("connect() requires the async OmniTunAsync client")
