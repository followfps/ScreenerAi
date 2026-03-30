import requests
import json
import time
import sys

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImU4YTlhNTYwLWNiZWUtNDUyMi05ZjQwLWZiNDQyODgzYmQyOCIsImxhc3RfcGFzc3dvcmRfY2hhbmdlIjoxNzUwNjYwODczLCJleHAiOjE3NzQ5ODc4NDN9.yUyqY6hDrmTi4GljN-wUj_BJQP13PFko4a6BEeDWuaY"

def test_registration(name, body):
    url = "https://chat.qwen.ai/api/v1/files"
    headers = {
        "Authorization": f"Bearer {TOKEN}",
        "Content-Type": "application/json",
        "X-Requested-With": "XMLHttpRequest"
    }
    try:
        resp = requests.post(url, headers=headers, json=body, timeout=10)
        print(f"Test: {name} | Status: {resp.status_code}")
        # Safely print response
        print(resp.text.encode('ascii', 'ignore').decode('ascii')[:500] + "\n")
    except Exception as e:
        print(f"Exception: {e}\n")

meta = {
    "file_id": "5e2dd3de-6ecf-4423-80b1-3a16f1d2a48d",
    "name": "test.png",
    "size": 1024,
    "type": "image",
    "category": "chat"
}

test_registration("Wrapped Object", {"file": meta})
test_registration("Wrapped Array", {"file": [meta]})
test_registration("Flat Object", meta)
test_registration("Flat Array", [meta])
test_registration("Files Plural", {"files": meta})
test_registration("Files Plural Array", {"files": [meta]})
