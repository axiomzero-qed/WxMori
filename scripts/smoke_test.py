"""Cross-client MCP regression / 独立客户端 MCP 回归。 No private data / 不使用真实数据。"""
import argparse
import hashlib
import json
import os
import pathlib
import re
import subprocess
import tempfile

from mcp_probe import Client


def run(command):
    with tempfile.TemporaryDirectory(prefix="wxmori-probe-") as temp:
        root = pathlib.Path(temp) / "合成 Demo"
        subprocess.run([command, "demo", "--dir", str(root)], check=True, capture_output=True)
        config = str(root / "config.json")
        for protocol in ("2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25"):
            c = Client(command, config, protocol)
            try:
                assert c.initialized["protocolVersion"] == protocol, c.initialized
                listed = c.request("tools/list", {})["tools"]
                assert len(listed) == 7
                for tool in listed:
                    schema = tool["inputSchema"]
                    assert schema["type"] == "object"
                    assert schema.get("additionalProperties") is False
                    for field in schema.get("properties", {}).values():
                        assert isinstance(field["type"], str), "Avoid union types in flat tool schemas"

                def call(name, args, error=None):
                    r = c.request("tools/call", {"name": name, "arguments": args})
                    data = r["structuredContent"]
                    assert json.loads(r["content"][0]["text"]) == data
                    if error:
                        assert r["isError"] is True and data["error"]["code"] == error, data
                    else:
                        assert not r.get("isError", False), data
                    return data

                assert call("wxmori_status", {})["ready"] is True
                assert call("capture_key", {"action": "status"})["state"] == "idle"
                call("capture_key", {"action": "invalid"}, "INVALID_ARGUMENTS")
                call("capture_key", {"action": "start"}, "CAPTURE_SOURCE_REQUIRED" if os.name=='nt' else "CAPTURE_UNSUPPORTED")
                sessions = call("list_sessions", {"keyword": "微忆"})["sessions"]
                assert len(sessions) == 1
                sid = sessions[0]["id"]
                first = call("get_messages", {"session_id": sid, "limit": 2})
                second = call("get_messages", {"session_id": sid, "limit": 2, "offset": first["next_offset"]})
                assert not {m["server_id"] for m in first["messages"]} & {m["server_id"] for m in second["messages"]}
                assert call("search_messages", {"query": "testing bob"})["total"] == 1
                assert call("get_contacts", {"query": "Alice"})["returned"] == 1
                call("export_chat", {"session_id": "Twin Demo"}, "AMBIGUOUS_SESSION")
                call("export_chat", {"session_id": sid, "months": ["2026-13"]}, "INVALID_MONTH")
                call("get_messages", {}, "INVALID_ARGUMENTS")
                call("get_messages", {"session_id": sid, "limit": "2"}, "INVALID_ARGUMENTS")
                call("get_messages", {"session_id": sid, "limit": -1}, "INVALID_PAGINATION")
                call("search_messages", {"query": "  "}, "QUERY_REQUIRED")
                bad = call("export_chat", {"session_id": sid, "output_dir": "outside"}, "INVALID_ARGUMENTS")
                assert "outside" not in json.dumps(bad)
                assert call("export_chat", {"session_id": sid, "months": ["2025-02"]})["files"] == []
                for format in ("txt", "jsonl"):
                    exported = call("export_chat", {"session_id": sid, "months": ["2026-09"], "format": format})
                    assert exported["message_count"] == 9
                    for f in exported["files"]:
                        content = pathlib.Path(f["path"]).read_bytes()
                        assert hashlib.sha256(content).hexdigest() == f["sha256"]
                        text = content.decode("utf-8")
                        lines = text.splitlines() if format == "jsonl" else re.findall(r"^\[\d{4}-\d{2}-\d{2} ", text, re.M)
                        assert len(lines) == 9
                assert call("wxmori_status", {})["snapshot_state"] == "ready"
                print(json.dumps({"protocol": protocol, "tool_count": len(listed), "result": "passed"}))
            finally:
                c.close()


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--command", required=True)
    run(str(pathlib.Path(p.parse_args().command).resolve()))
