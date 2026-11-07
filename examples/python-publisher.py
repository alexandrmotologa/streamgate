#!/usr/bin/env python3
"""
streamgate Python Publisher Example
Uses only the standard library (urllib.request) to publish events to streamgate.
"""

import json
import os
import random
import time
import urllib.request

GATEWAY_URL = os.getenv("STREAMGATE_URL", "http://localhost:8080/api/v1/publish")
CHANNEL = os.getenv("STREAMGATE_CHANNEL", "finance-trades")


def publish_event(channel: str, event: str, data: dict):
    payload = json.dumps({
        "channel": channel,
        "event": event,
        "data": data,
    }).encode("utf-8")

    req = urllib.request.Request(
        GATEWAY_URL,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(req) as resp:
            result = json.loads(resp.read().decode("utf-8"))
            print(f"[PUBLISHED] Event ID: {result.get('id')} | Delivered to: {result.get('delivered_to')} subscribers")
    except Exception as e:
        print(f"[ERROR] Publish request failed: {e}")


if __name__ == "__main__":
    print(f"Broadcasting Python events to {GATEWAY_URL} on channel '{CHANNEL}'...")
    count = 0
    try:
        while True:
            count += 1
            payload = {
                "symbol": "ETH/USD",
                "price": round(2800.0 + random.uniform(-10.0, 10.0), 2),
                "volume": round(random.uniform(0.1, 5.0), 3),
                "iteration": count,
                "timestamp": int(time.time() * 1000),
            }
            publish_event(CHANNEL, "trade.executed", payload)
            time.sleep(1.0)
    except KeyboardInterrupt:
        print("\nPublisher stopped.")
