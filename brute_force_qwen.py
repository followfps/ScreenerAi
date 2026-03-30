import requests
import json
import time

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImU4YTlhNTYwLWNiZWUtNDUyMi05ZjQwLWZiNDQyODgzYmQyOCIsImxhc3RfcGFzc3dvcmRfY2hhbmdlIjoxNzUwNjYwODczLCJleHAiOjE3NzQ5ODc4NDN9.yUyqY6hDrmTi4GljN-wUj_BJQP13PFko4a6BEeDWuaY"

def test_payload(files_array_or_element):
    headers = {
        "Authorization": f"Bearer {TOKEN}",
        "Content-Type": "application/json",
        "User-Agent": "Mozilla/5.0",
        "Origin": "https://chat.qwen.ai",
        "Referer": "https://chat.qwen.ai/"
    }
    
    payload = {
        "stream": False,
        "chat_id": f"test-chat-{int(time.time())}",
        "chat_mode": "normal",
        "model": "qwen3-vl-plus",
        "messages": [
            {
                "role": "user",
                "content": "test message",
                "chat_type": "t2v",
                "sub_chat_type": "t2v",
                "files": files_array_or_element
            }
        ]
    }

    url = "https://chat.qwen.ai/api/v2/chat/completions"
    try:
        resp = requests.post(url, headers=headers, json=payload, timeout=10)
        print(f"Payload: {json.dumps(files_array_or_element)} | Status: {resp.status_code} | Body: {resp.text[:200]}")
    except Exception as e:
        print(f"Exception: {e}")

variants = [
    # 1. Just strings (this should fail)
    ["file-uuid"],
    # 2. {file_id: string}
    [{"file_id": "file-uuid"}],
    # 3. {id: string}
    [{"id": "file-uuid"}],
    # 4. {file: string}
    [{"file": "file-uuid"}],
    # 5. {name, file_id}
    [{"file_id": "file-uuid", "name": "test.png", "size": 100}],
    # 6. {url}
    [{"url": "https://fake.url/image.png"}],
    # 7. {file_path}
    [{"file_path": "fake/path/image.png", "file_id": "file-uuid"}],
    # 8. All metadata
    [{"name":"test.png","url":"https://fake","id":"file-uuid","size":124,"type":"image","file_id":"file-uuid","file_path":"fake","region":"fake","bucketname":"fake"}]
]

for v in variants:
    test_payload(v)
    time.sleep(1)
