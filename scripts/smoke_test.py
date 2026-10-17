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
        home = pathlib.Path(temp) / "synthetic-home"
        home.mkdir()
        previous_home = os.environ.get("HOME")
        previous_profile = os.environ.get("USERPROFILE")
        os.environ["HOME"] = str(home)
        os.environ["USERPROFILE"] = str(home)
        try:
            # Account discovery is tested with an empty synthetic directory;
            # the demo itself is decrypted data and has no encrypted account.
            (home / "Documents" / "xwechat_files" / "wxid_smoke" / "db_storage").mkdir(parents=True)
            root = pathlib.Path(temp) / "合成 Demo"
            subprocess.run([command, "demo", "--dir", str(root)], check=True, capture_output=True)
            config = str(root / "config.json")
            protocols = ("2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25")
            expected_tools = {
                "wxmori_status", "list_accounts", "capture_key", "list_sessions",
                "get_messages", "search_messages", "get_contacts", "export_chat",
                "export_all_chats",
            }
            for protocol in protocols:
                c = Client(command, config, protocol)
                try:
                    assert c.initialized["protocolVersion"] == protocol, c.initialized
                    listed = c.request("tools/list", {})["tools"]
                    assert len(listed) == 9
                    by_name = {tool["name"]: tool for tool in listed}
                    assert set(by_name) == expected_tools
                    for tool in listed:
                        schema = tool["inputSchema"]
                        assert schema["type"] == "object"
                        assert schema.get("additionalProperties") is False
                        for field in schema.get("properties", {}).values():
                            field_type = field["type"]
                            assert isinstance(field_type, str), "Avoid union types in flat tool schemas"
                            if field_type == "array":
                                assert field["items"]["type"] == "string", "Array tool fields need a flat item type"

                    inspect_schema = by_name["wxmori_status"]["inputSchema"]
                    inspect_field = inspect_schema["properties"]["inspect"]
                    assert inspect_field["type"] == "boolean"
                    assert "inspect" not in inspect_schema.get("required", [])

                    def call(name, args, error=None):
                        response = c.request("tools/call", {"name": name, "arguments": args})
                        data = response["structuredContent"]
                        assert json.loads(response["content"][0]["text"]) == data
                        if error:
                            assert response["isError"] is True and data["error"]["code"] == error, data
                        else:
                            assert not response.get("isError", False), data
                        return data

                    assert call("wxmori_status", {})["ready"] is True
                    inspected = call("wxmori_status", {"inspect": True})
                    whoami = inspected["whoami"]
                    assert whoami["contact_match_verified"] is False, whoami
                    assert whoami.get("display_name") is None and whoami.get("wechat_alias") is None, whoami
                    assert "wxid_demo_alice" not in json.dumps(whoami) and "wxid_demo_bob" not in json.dumps(whoami), whoami
                    assert inspected["history"]["session_count"] >= 1
                    assert isinstance(inspected["diagnostics"], list)

                    accounts = call("list_accounts", {"limit": 1, "offset": 0})
                    assert accounts["total"] == 1 and accounts["returned"] == 1 and accounts["has_more"] is False, accounts
                    empty_page = call("list_accounts", {"limit": 1, "offset": 1})
                    assert empty_page["total"] == 1 and empty_page["returned"] == 0 and empty_page["offset"] == 1
                    call("list_accounts", {"limit": "1"}, "INVALID_ARGUMENTS")
                    call("wxmori_status", {"inspect": "true"}, "INVALID_ARGUMENTS")

                    assert call("capture_key", {"action": "status"})["state"] == "idle"
                    call("capture_key", {}, "INVALID_ARGUMENTS")
                    call("capture_key", {"action": "invalid"}, "INVALID_ARGUMENTS")
                    call("capture_key", {"action": "start"}, "CAPTURE_SOURCE_REQUIRED" if os.name == "nt" else "CAPTURE_UNSUPPORTED")
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
                    for export_format in ("txt", "jsonl"):
                        exported = call("export_chat", {"session_id": sid, "months": ["2026-09"], "format": export_format})
                        assert exported["message_count"] == 9
                        for file in exported["files"]:
                            content = pathlib.Path(file["path"]).read_bytes()
                            assert hashlib.sha256(content).hexdigest() == file["sha256"]
                            text = content.decode("utf-8")
                            rows = text.splitlines() if export_format == "jsonl" else re.findall(r"^\[\d{4}-\d{2}-\d{2} ", text, re.M)
                            assert len(rows) == 9

                    call("export_all_chats", {"months": "2026-09"}, "INVALID_ARGUMENTS")
                    call("export_all_chats", {"months": ["2026-13"], "format": "jsonl"}, "INVALID_MONTH")
                    call("export_all_chats", {"from": "2026-10-01", "to": "2026-09-01"}, "INVALID_DATE_RANGE")
                    all_dates = call("export_all_chats", {})
                    assert all_dates["session_count"] == 3
                    batch = call("export_all_chats", {"months": ["2026-09"], "format": "jsonl"})
                    assert batch["session_count"] == 3 and batch["message_count"] == 9
                    manifest_bytes = pathlib.Path(batch["manifest_path"]).read_bytes()
                    assert hashlib.sha256(manifest_bytes).hexdigest() == batch["manifest_sha256"]
                    manifest = json.loads(manifest_bytes)
                    assert manifest["message_count"] == 9 and manifest["session_count"] == 3
                    row_total = 0
                    file_total = 0
                    for session in manifest["sessions"]:
                        for file in session["files"]:
                            content = pathlib.Path(file["path"]).read_bytes()
                            assert hashlib.sha256(content).hexdigest() == file["sha256"]
                            rows = [json.loads(line) for line in content.decode("utf-8").splitlines()]
                            assert len(rows) == file["message_count"]
                            row_total += len(rows)
                            file_total += 1
                    assert row_total == 9 and file_total == batch["file_count"]

                    assert call("wxmori_status", {})["snapshot_state"] == "ready"
                    print(json.dumps({"protocol": protocol, "tool_count": len(listed), "result": "passed"}))
                finally:
                    c.close()
        finally:
            if previous_home is None:
                os.environ.pop("HOME", None)
            else:
                os.environ["HOME"] = previous_home
            if previous_profile is None:
                os.environ.pop("USERPROFILE", None)
            else:
                os.environ["USERPROFILE"] = previous_profile


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--command", required=True)
    run(str(pathlib.Path(p.parse_args().command).resolve()))
