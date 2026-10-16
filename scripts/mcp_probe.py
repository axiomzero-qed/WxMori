"""Small independent stdio MCP client / 独立的 stdio MCP 测试客户端。"""
import argparse
import json
import queue
import subprocess
import sys
import threading


class Client:
    def __init__(self, command, config, protocol="2025-06-18"):
        self.process = subprocess.Popen(
            [command, "--config", config, "serve"],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, encoding="utf-8", bufsize=1,
        )
        self.lines = queue.Queue()
        self.serial = 0
        self.errors = []
        def read_stdout():
            for line in self.process.stdout:
                self.lines.put(line)
            self.lines.put(None)
        def read_stderr():
            for line in self.process.stderr:
                self.errors.append(line)
        threading.Thread(target=read_stdout, daemon=True).start()
        threading.Thread(target=read_stderr, daemon=True).start()
        self.initialized = self.request("initialize", {
            "protocolVersion": protocol, "capabilities": {},
            "clientInfo": {"name": "wxmori-python-probe", "version": "1"},
        })
        self.notify("notifications/initialized", {})

    def notify(self, method, params):
        self.process.stdin.write(json.dumps({"jsonrpc": "2.0", "method": method, "params": params}) + "\n")
        self.process.stdin.flush()

    def request(self, method, params):
        self.serial += 1
        self.process.stdin.write(json.dumps({"jsonrpc": "2.0", "id": self.serial, "method": method, "params": params}) + "\n")
        self.process.stdin.flush()
        while True:
            line = self.lines.get(timeout=180)
            if line is None:
                raise RuntimeError("MCP server closed stdout: " + "".join(self.errors))
            response = json.loads(line)
            if response.get("id") == self.serial:
                if "error" in response:
                    raise RuntimeError(response["error"])
                return response["result"]

    def close(self):
        self.process.stdin.close()
        try:
            self.process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.process.terminate()
            self.process.wait(timeout=10)
        self.process.stdout.close()
        self.process.stderr.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--command", required=True)
    parser.add_argument("--config", required=True)
    parser.add_argument("--protocol", default="2025-06-18")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--list-tools", action="store_true")
    group.add_argument("--tool")
    parser.add_argument("--args", default="{}")
    args = parser.parse_args()
    client = Client(args.command, args.config, args.protocol)
    try:
        if args.list_tools:
            output = client.request("tools/list", {})
        else:
            output = client.request("tools/call", {"name": args.tool, "arguments": json.loads(args.args)})
        sys.stdout.reconfigure(encoding="utf-8")
        print(json.dumps(output, ensure_ascii=False, indent=2))
    finally:
        client.close()


if __name__ == "__main__":
    main()
