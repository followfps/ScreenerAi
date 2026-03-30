import requests
import json
import time

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImU4YTlhNTYwLWNiZWUtNDUyMi05ZjQwLWZiNDQyODgzYmQyOCIsImxhc3RfcGFzc3dvcmRfY2hhbmdlIjoxNzUwNjYwODczLCJleHAiOjE3NzQ5ODc4NDN9.yUyqY6hDrmTi4GljN-wUj_BJQP13PFko4a6BEeDWuaY"
CHAT_ID = "7e0a63d0-d0d7-436a-aab6-d051ce40c289" # From user logs
PARENT_ID = "02805801-da3b-408b-b394-357c7b0eee54" # From user logs

def test_payload(files_array, chat_type="t2v"):
    headers = {
        "Authorization": f"Bearer {TOKEN}",
        "Content-Type": "application/json",
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
        "Origin": "https://chat.qwen.ai",
        "Referer": "https://chat.qwen.ai/",
        "X-Requested-With": "XMLHttpRequest"
    }
    
    now = int(time.time())
    payload = {
        "stream": False,
        "chat_id": CHAT_ID,
        "chat_mode": "normal",
        "model": "qwen3-vl-plus",
        "messages": [
            {
                "fid": "test-fid-" + str(now),
                "role": "user",
                "content": "What is in this image?",
                "chat_type": chat_type,
                "sub_chat_type": chat_type,
                "files": files_array,
                "timestamp": now,
                "user_action": "chat",
                "models": ["qwen3-vl-plus"]
            }
        ],
        "timestamp": now
    }

    url = f"https://chat.qwen.ai/api/v2/chat/completions?chat_id={CHAT_ID}"
    try:
        resp = requests.post(url, headers=headers, json=payload, timeout=20)
        print(f"ChatType: {chat_type} | Files: {json.dumps(files_array)} | Status: {resp.status_code}")
        print(f"Body: {resp.text[:500]}\n")
    except Exception as e:
        print(f"Exception: {e}\n")

variants = [
    # Verify if URL dict works
    ([{"url": "https://help.aliyun.com/favicon.ico"}], "t2t"),
    ([{"url": "https://help.aliyun.com/favicon.ico"}], "t2v"),
    ([{"url": "https://help.aliyun.com/favicon.ico"}], "v2t"),
    # Verify if file_id dict works
    ([{"file_id": "8d475c86-eb8b-4896-ae4e-b14800bd0efc"}], "t2v"), # Using a random request id as fake file_id
]

for v, ct in variants:
    test_payload(v, ct)
    time.sleep(1)
