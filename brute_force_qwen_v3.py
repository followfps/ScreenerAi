import requests
import json
import time
import uuid

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImU4YTlhNTYwLWNiZWUtNDUyMi05ZjQwLWZiNDQyODgzYmQyOCIsImxhc3RfcGFzc3dvcmRfY2hhbmdlIjoxNzUwNjYwODczLCJleHAiOjE3NzQ5ODc4NDN9.yUyqY6hDrmTi4GljN-wUj_BJQP13PFko4a6BEeDWuaY"

def create_chat(model):
    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    payload = {"model": model, "title": "Test Chat"}
    resp = requests.post("https://chat.qwen.ai/api/v2/chat/create", headers=headers, json=payload)
    if resp.status_code == 200:
        return resp.json().get("data", {}).get("chat_id")
    return None

def test_payload(chat_id, files_array, chat_type="t2t"):
    headers = {
        "Authorization": f"Bearer {TOKEN}",
        "Content-Type": "application/json",
        "User-Agent": "Mozilla/5.0",
        "X-Requested-With": "XMLHttpRequest"
    }
    
    now = int(time.time())
    payload = {
        "stream": False,
        "chat_id": chat_id,
        "chat_mode": "normal",
        "model": "qwen3-vl-plus",
        "messages": [
            {
                "fid": uuid.uuid4().hex[:16],
                "role": "user",
                "content": "What is this?",
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

    url = f"https://chat.qwen.ai/api/v2/chat/completions?chat_id={chat_id}"
    try:
        resp = requests.post(url, headers=headers, json=payload, timeout=20)
        print(f"ChatType: {chat_type} | FilesLen: {len(files_array)} | Status: {resp.status_code}")
        print(f"Body: {resp.text[:500]}\n")
    except Exception as e:
        print(f"Exception: {e}\n")

model = "qwen3-vl-plus"
cid = create_chat(model)
if not cid:
    print("Failed to create chat")
    exit(1)

print(f"Created chat: {cid}")

# Use a real URL found in common Aliyun examples or just a fake one with full metadata
full_metadata = {
    "name": "screenshot.png",
    "url": "https://help.aliyun.com/favicon.ico",
    "id": "file-123",
    "file_id": "file-123",
    "size": 1024,
    "type": "image"
}

variants = [
    ([full_metadata], "t2t"),
    ([full_metadata], "multimodal"),
]

for v, ct in variants:
    test_payload(cid, v, ct)
    time.sleep(1)
