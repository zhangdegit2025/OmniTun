from __future__ import annotations

import json
import time
import urllib.request
import urllib.error
from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Optional

DEFAULT_BASE_URL = "https://api.omnitun.dev"
DEFAULT_USER_AGENT = "omnitun-python"
DEFAULT_RETRIES = 3
DEFAULT_RETRY_DELAY = 0.5


class OmniTunError(Exception):
    def __init__(self, code: str, message: str, details: Any = None):
        self.code = code
        self.message = message
        self.details = details
        super().__init__(message)


class TunnelProtocol(str, Enum):
    TCP = "tcp"
    HTTP = "http"
    HTTPS = "https"


class TunnelStatus(str, Enum):
    ACTIVE = "active"
    STOPPED = "stopped"
    ERROR = "error"


@dataclass
class Tunnel:
    id: str
    name: str
    protocol: str
    local_port: int
    remote_port: int
    status: str
    traffic_in: int = 0
    traffic_out: int = 0
    domain: Optional[str] = None
    tags: list[str] = field(default_factory=list)
    auth_mode: Optional[str] = None
    tls_mode: Optional[str] = None
    compression: bool = False
    created_at: Optional[str] = None
    updated_at: Optional[str] = None


@dataclass
class Domain:
    id: str
    domain: str
    verified: bool = False
    tunnel_id: Optional[str] = None
    verification: Optional[str] = None
    created_at: Optional[str] = None


@dataclass
class MeshNetwork:
    id: str
    name: str
    cidr: str
    node_count: int = 0
    created_at: Optional[str] = None


@dataclass
class MeshNode:
    id: str
    network_id: str
    name: str
    ip_address: str
    public_key: str
    nat_type: str = ""
    endpoints: list[str] = field(default_factory=list)
    status: str = "offline"
    last_seen: Optional[str] = None
    created_at: Optional[str] = None


class OmniTun:
    def __init__(
        self,
        token: str,
        base_url: str = DEFAULT_BASE_URL,
        user_agent: str = DEFAULT_USER_AGENT,
        timeout: float = 30.0,
        max_retries: int = DEFAULT_RETRIES,
    ):
        self.token = token
        self.base_url = base_url.rstrip("/")
        self.user_agent = user_agent
        self.max_retries = max_retries
        self.timeout = timeout
        self.tunnels = TunnelsAPI(self)

    def request(self, method: str, path: str, json_body: Any = None) -> Any:
        url = f"{self.base_url}{path}"
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
            "User-Agent": self.user_agent,
            "Accept": "application/json",
        }

        data: Optional[bytes] = None
        if json_body is not None:
            data = json.dumps(json_body).encode("utf-8")

        last_exc: Optional[Exception] = None
        for attempt in range(self.max_retries):
            try:
                req = urllib.request.Request(
                    url,
                    data=data,
                    headers=headers,
                    method=method.upper(),
                )
                resp = urllib.request.urlopen(req, timeout=self.timeout)

                if resp.status == 429:
                    last_exc = OmniTunError("RATE_LIMITED", "Too many requests")
                    time.sleep(DEFAULT_RETRY_DELAY * (attempt + 1))
                    continue

                if resp.status >= 500:
                    last_exc = OmniTunError(
                        f"HTTP_{resp.status}", f"Server error: {resp.status}"
                    )
                    if attempt < self.max_retries - 1:
                        time.sleep(DEFAULT_RETRY_DELAY * (attempt + 1))
                    continue

                if resp.status >= 400:
                    body = resp.read()
                    try:
                        body_json = json.loads(body)
                        code = body_json.get("code", f"HTTP_{resp.status}")
                        msg = body_json.get("message", body.decode("utf-8"))
                        raise OmniTunError(code, msg, body_json.get("details"))
                    except json.JSONDecodeError:
                        raise OmniTunError(
                            f"HTTP_{resp.status}", body.decode("utf-8", errors="replace")
                        )

                if resp.status == 204:
                    return None

                body = resp.read()
                return json.loads(body)

            except urllib.error.HTTPError as e:
                try:
                    body = e.read()
                    body_json = json.loads(body)
                    code = body_json.get("code", f"HTTP_{e.code}")
                    msg = body_json.get("message", str(e))
                    raise OmniTunError(code, msg, body_json.get("details"))
                except (json.JSONDecodeError, OmniTunError):
                    raise OmniTunError(f"HTTP_{e.code}", str(e))
            except urllib.error.URLError as e:
                last_exc = e
                if attempt < self.max_retries - 1:
                    time.sleep(DEFAULT_RETRY_DELAY * (attempt + 1))
                continue

        if last_exc:
            raise OmniTunError(
                "REQUEST_FAILED",
                f"Request failed after {self.max_retries} retries: {last_exc}",
            )
        raise OmniTunError("REQUEST_FAILED", "Request failed")

    def close(self):
        pass

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()


from .tunnels import TunnelsAPI

__all__ = [
    "OmniTun",
    "OmniTunError",
    "Tunnel",
    "TunnelProtocol",
    "TunnelStatus",
    "Domain",
    "MeshNetwork",
    "MeshNode",
]
