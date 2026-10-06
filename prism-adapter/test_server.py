import importlib.util
import json
import tempfile
import threading
import unittest
from pathlib import Path
from http.server import HTTPServer
from urllib.error import HTTPError
from urllib.request import Request, urlopen


spec = importlib.util.spec_from_file_location("prism_adapter", Path(__file__).with_name("server.py"))
adapter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adapter)


class AdapterTests(unittest.TestCase):
    def test_supported_models_are_forwarded_without_aliasing(self):
        for model in adapter.MODELS:
            prompt, stream = adapter.parse_prompt({"model": model, "input": "hi"})
            self.assertEqual(prompt, "[user]\nhi")
            self.assertFalse(stream)

    def test_text_request_keeps_model_and_stream(self):
        prompt, stream = adapter.parse_prompt({
            "model": "gpt-5.6-sol", "stream": True,
            "instructions": "Answer exactly.",
            "input": [{"role": "user", "content": [{"type": "input_text", "text": "hi"}]}],
        })
        self.assertEqual(prompt, "[instructions]\nAnswer exactly.\n\n[user]\nhi")
        self.assertTrue(stream)

    def test_unsupported_features_fail_closed(self):
        for change in ({"model": "gpt-6-astra"}, {"tools": [{"type": "function", "name": "x"}]},
                       {"previous_response_id": "resp_1"}, {"reasoning": {"effort": "high"}}):
            request = {"model": "gpt-5.6-sol", "input": "hi", **change}
            with self.assertRaises(adapter.AdapterError):
                adapter.parse_prompt(request)

    def test_terminal_output_and_unknown_state(self):
        self.assertIsNone(adapter.terminal_text({"status": "running"}))
        self.assertEqual(adapter.terminal_text({"status": "completed", "response": {
            "status": "success", "payload": {"output": [{"type": "message", "content": [{"text": "21"}]}]}
        }}), "21")

    def test_pending_journal_blocks_ambiguous_replay(self):
        with tempfile.TemporaryDirectory() as directory:
            state = adapter.State(directory)
            state.begin("300")
            with self.assertRaises(adapter.AdapterError) as raised:
                state.begin("300")
            self.assertEqual(raised.exception.status, 409)
            state.finish("300")
            state.begin("300")
            state.finish("300")

    def test_http_boundary_uses_real_terminal_without_usage(self):
        class FakeBrowser:
            def run(self, account_id, token, prompt, model):
                self.assert_values = (account_id, token, prompt, model)
                return "prism-123", "21"

        fake = FakeBrowser()
        handler = type("TestHandler", (adapter.Handler,), {"api_key": "test-key", "browser_turn": fake})
        server = HTTPServer(("127.0.0.1", 0), handler)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            url = f"http://127.0.0.1:{server.server_port}/v1/responses"
            data = json.dumps({"model": "gpt-5.6-sol", "input": "candy"}).encode()
            headers = {"Authorization": "Bearer test-key", "X-Prism-Account-ID": "300",
                       "X-Prism-OAuth-Token": "oauth-token", "Content-Type": "application/json"}
            with urlopen(Request(url, data=data, headers=headers), timeout=5) as response:
                body = json.load(response)
            self.assertEqual(fake.assert_values, ("300", "oauth-token", "[user]\ncandy", "gpt-5.6-sol"))
            self.assertEqual(body["output"][0]["content"][0]["text"], "21")
            self.assertNotIn("usage", body)
            with self.assertRaises(HTTPError) as denied:
                urlopen(Request(url, data=data, headers={"Content-Type": "application/json"}), timeout=5)
            self.assertEqual(denied.exception.code, 401)
        finally:
            server.shutdown()
            server.server_close()


if __name__ == "__main__":
    unittest.main()
