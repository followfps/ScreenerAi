import requests
import json
import time

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6ImU4YTlhNTYwLWNiZWUtNDUyMi05ZjQwLWZiNDQyODgzYmQyOCIsImxhc3RfcGFzc3dvcmRfY2hhbmdlIjoxNzUwNjYwODczLCJleHAiOjE3NzQ5ODc4NDN9.yUyqY6hDrmTi4GljN-wUj_BJQP13PFko4a6BEeDWuaY"
CHAT_ID = "7403e46a-c985-4fc0-a640-e21e1bc7cb72"

def test_payload(name, modifications):
    # Base payload from the user's RAW REQUEST
    payload = {
        "chat_id": CHAT_ID,
        "chat_mode": "normal",
        "incremental_output": True,
        "messages": [
            {
                "chat_type": "multimodal",
                "childrenIds": ["8c57ee4f1f16e1c90c054f0fff53cfb5"],
                "content": "What is in this image?",
                "extra": {"meta": {"subChatType": "t2t"}},
                "feature_config": {"output_schema": "phase", "thinking_enabled": False},
                "fid": "a895a16183d62ece5527de5287cb0c48",
                "files": [
                    {
                        "file_id": "e3218611-e6ba-4883-9716-98ddb3135642",
                        "id": "e3218611-e6ba-4883-9716-98ddb3135642",
                        "type": "image",
                        "url": "https://qwen-webui-prod.oss-accelerate.aliyuncs.com/e8a9a560-cbee-4522-9f40-fb442883bd28/e3218611-e6ba-4883-9716-98ddb3135642_1774802795770-561b819d058d8db8-screenshot.png?x-oss-security-token=CAIS0AJ1q6Ft5B2yfSjIr5rtPPzgtIwSg6%2FaR3zch3BnSbxYp%2Fbauzz2IHhMf3RvBeAbs%2Fs1lWBZ7vwflrN6SJtIXleCZtF94plR7QKoZ73Zocur7LAJksVmppcbsEWpsvXJasDVEfn%2FGJ70GX2m%2BwZ3xbzlD0bAO3WuLZyOj7N%2Bc90TRXPWRDFaBdBQVGAAwY1gQhm3D%2Fu2NQPwiWf9FVdhvhEG6Vly8qOi2MaRmHG85R%2FYsrZN%2BNmgecP%2FNpE3bMwiCYyPsbYoJvab4kl58ANX8ap6tqtA9Arcs8uVa1sruE3eaLeLro0ycVAjN%2FhrQ%2FQZtpn1lvl1ofeWkJznAJW0o2rsz001LaPXI6uscIvBXr5R%2FoZuLfsAcX6lIniYYa61rD9q9eLnNgjPEHUE7TZGpF9U5FEJVv4kDKnd5WarfsNS895Bq3%2BMm37iOgxDnerxuU4agAFmSwzFWlDP7oEfm8rJLRNQ3hB8gt1yPfJQLl88Yk6CSo90ysfbmBI97pnc6qiST9Hw7clCh4neGokRpR3UAbakg4DmmjB%2BrU5%2FmL%2BF1TXRdrAx0VKRf6kQUKTz80AenUml6GKOpLwrEJoGEgopXFKN1gv3V%2Bfzl148p8pL73HDfiAA&x-oss-date=20260329T164636Z&x-oss-expires=300&x-oss-signature-version=OSS4-HMAC-SHA256&x-oss-credential=STS.NYXwFTYS34m1EZmcp2E3tH3qY%2F20260329%2Fap-southeast-1%2Foss%2Faliyun_v4_request&x-oss-signature=e92c119fb82aae8b05a82395a1e730f5b9238be7d1900117620320084f651277"
                    }
                ],
                "models": ["qwen3-vl-plus"],
                "parentId": None,
                "parent_id": None,
                "role": "user",
                "sub_chat_type": "multimodal",
                "timestamp": int(time.time()),
                "user_action": "chat"
            }
        ],
        "model": "qwen3-vl-plus",
        "parent_id": None,
        "stream": False,
        "timestamp": int(time.time())
    }
    
    for key, value in modifications.items():
        if "." in key:
            parts = key.split(".")
            target = payload["messages"][0]
            for p in parts[:-1]:
                target = target[p]
            target[parts[-1]] = value
        else:
            payload[key] = value

    url = f"https://chat.qwen.ai/api/v2/chat/completions?chat_id={CHAT_ID}"
    headers = {
        "Authorization": f"Bearer {TOKEN}",
        "Content-Type": "application/json",
        "User-Agent": "Mozilla/5.0",
        "X-Requested-With": "XMLHttpRequest"
    }
    
    try:
        resp = requests.post(url, headers=headers, json=payload, timeout=10)
        print(f"Test: {name} | Status: {resp.status_code}")
        print(f"Body: {resp.text[:500]}\n")
    except Exception as e:
        print(f"Exception: {e}\n")

tests = [
    ("Control (Baseline)", {}),
    ("Fix SubChatType", {"messages.0.extra.meta.subChatType": "multimodal"}),
    ("Remove FeatureConfig", {"messages.0.feature_config": {}}),
    ("Add Placeholder", {"messages.0.content": "[file_0]\nWhat is this?"}),
    ("All in One", {
        "messages.0.extra.meta.subChatType": "multimodal",
        "messages.0.feature_config": {},
        "messages.0.content": "[file_0]\nWhat is this?"
    })
]

for name, mods in tests:
    test_payload(name, mods)
    time.sleep(1)
