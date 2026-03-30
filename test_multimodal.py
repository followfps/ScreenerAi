import requests
import json
import time

def create_png():
    # Larger 100x100 white PNG
    import zlib
    def chunk(type, data):
        return (len(data).to_bytes(4, 'big') + type + data + 
                zlib.crc32(type + data).to_bytes(4, 'big'))
    
    png = b'\x89PNG\r\n\x1a\n'
    png += chunk(b'IHDR', (100).to_bytes(4, 'big') + (100).to_bytes(4, 'big') + 
                 b'\x08\x02\x00\x00\x00')
    png += chunk(b'IDAT', zlib.compress(b'\xff' * 30000))
    png += chunk(b'IEND', b'')
    with open("test_image.png", "wb") as f:
        f.write(png)

def test_multimodal():
    create_png()
    
    # 1. Upload a test file to get a URL and FileID
    upload_url = "http://localhost:3264/api/files/upload"
    with open("test_image.png", "rb") as f:
        resp = requests.post(upload_url, files={"file": f})
    
    if resp.status_code != 200:
        print(f"Upload failed: {resp.text}")
        return
    
    upload_data = resp.json()
    file_id = upload_data["file"]["id"]
    file_url = upload_data["file"]["url"]
    print(f"Uploaded: ID={file_id}, URL={file_url[:50]}...")

    # 2. Send chat completion request
    chat_url = "http://localhost:3264/api/chat/completions"
    payload = {
        "model": "qwen3-vl-plus",
        "messages": [
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": "What color is this image?"},
                    {"type": "image_url", "image_url": {"url": file_url, "file_id": file_id}}
                ]
            }
        ],
        "stream": False
    }
    
    print("Sending chat request...")
    resp = requests.post(chat_url, json=payload)
    print(f"Status: {resp.status_code}")
    print(f"Response: {resp.text}")

if __name__ == "__main__":
    test_multimodal()
