import requests
import json

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImU4YTlhNTYwLWNiZWUtNDUyMi05ZjQwLWZiNDQyODgzYmQyOCIsImxhc3RfcGFzc3dvcmRfY2hhbmdlIjoxNzUwNjYwODczLCJleHAiOjE3NzQ5ODc4NDN9.yUyqY6hDrmTi4GljN-wUj_BJQP13PFko4a6BEeDWuaY"

def test_registration(file_id):
    url = "https://chat.qwen.ai/api/v1/files"
    headers = {
        "Authorization": f"Bearer {TOKEN}",
        "X-Requested-With": "XMLHttpRequest",
        "Origin": "https://chat.qwen.ai",
        "Referer": "https://chat.qwen.ai/",
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
    }
    
    meta = {
        "file_id": file_id,
        "name": "test.png",
        "size": 69,
        "type": "image",
        "category": "chat"
    }
    
    print(f"Testing registration for ID: {file_id}")
    
    # 1. Try JSON Wrapped
    print("\n--- Trying JSON Wrapped {'file': meta} ---")
    resp = requests.post(url, headers=headers, json={"file": meta})
    print(f"Status: {resp.status_code} | Body: {resp.text}")

    # 2. Try JSON Wrapped Array
    print("\n--- Trying JSON Wrapped Array {'file': [meta]} ---")
    resp = requests.post(url, headers=headers, json={"file": [meta]})
    print(f"Status: {resp.status_code} | Body: {resp.text}")

    # 3. Try Form Data (Multipart but no file)
    print("\n--- Trying Form Data ---")
    fields = {
        "file_id": (None, file_id),
        "name": (None, "test.png"),
        "size": (None, "69"),
        "type": (None, "image"),
        "category": (None, "chat")
    }
    resp = requests.post(url, headers=headers, files=fields)
    print(f"Status: {resp.status_code} | Body: {resp.text}")

if __name__ == "__main__":
    # Get a fresh file_id via STS from the previous run or use a new one
    # I'll just run test_multimodal.py first to see what ID it generates
    test_registration("cbeb32d5-9b2d-454c-8004-30c823250f78")
