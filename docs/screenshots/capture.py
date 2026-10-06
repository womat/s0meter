"""Capture the README screenshots and the social preview of the s0meter web UI.

Serves the real app/ui/index.html with a mocked /health (sample meters whose pulses advance on
every request, so the pulse LEDs flash) and photographs it with headless Chromium. Run from the
project root, on demand only:

    docker run --rm -v "$PWD":/src -w /src mcr.microsoft.com/playwright/python:v1.52.0-noble \
        python3 docs/screenshots/capture.py

Writes docs/screenshots/web-ui*.png and docs/social-preview.png.
"""

import base64
import json
import threading
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parents[2]
PAGE = (ROOT / "app/ui/index.html").read_bytes()
OUT = ROOT / "docs/screenshots"
TZ = timezone(timedelta(hours=2))
PORT = 8765

# Sample meters: pulses added per request (one request every 3 s), meter constant and units.
METERS = {
    "wallbox": dict(gpio=17, pulses=34342123, step=6, ppu=1, unit="kWh", cdiv=1000, cprec=3,
                    gauge=7215.85, gunit="W", gprec=2, last=0.4),
    "drinkingwater": dict(gpio=22, pulses=1024317, step=1, ppu=1000, unit="m³", cdiv=1, cprec=3,
                          gauge=0.142, gunit="l/s", gprec=3, last=1.8),
    "greywater": dict(gpio=27, pulses=105459, step=0, ppu=1000, unit="m³", cdiv=1, cprec=3,
                      gauge=0, gunit="l/h", gprec=0, last=None),
}
UPTIME = 3 * 86400 + 4 * 3600 + 12 * 60


def health():
    now = datetime.now(TZ).replace(microsecond=0)
    meters = {}
    for name, m in METERS.items():
        m["pulses"] += m["step"]
        last = None if m["last"] is None else (now - timedelta(seconds=m["last"])).isoformat()
        meters[name] = {
            "gpio": m["gpio"],
            "pulses": m["pulses"],
            "lastPulse": last,
            "lastPulseAgeSeconds": m["last"],
            "droppedEvents": 0,
            "display": {
                "counter": round(m["pulses"] / m["ppu"] / m["cdiv"], m["cprec"]),
                "counterUnit": m["unit"],
                "counterPrecision": m["cprec"],
                "gauge": m["gauge"],
                "gaugeUnit": m["gunit"],
                "gaugePrecision": m["gprec"],
            },
        }
    return {
        "app": "s0meter", "appVersion": "5.0.0", "hostname": "pi-zero", "os": "linux",
        "uptimeSeconds": UPTIME, "mqtt": "connected", "meters": meters,
    }


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/":
            body, ctype = PAGE, "text/html; charset=utf-8"
        elif self.path == "/health":
            body, ctype = json.dumps(health()).encode(), "application/json"
        else:
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", ctype)
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def shoot(browser, path, width, height, scheme, scale=1, full_page=True):
    ctx = browser.new_context(viewport={"width": width, "height": height},
                              color_scheme=scheme, device_scale_factor=scale)
    ctx.add_init_script("localStorage.setItem('s0meter.apiKey', 'demo')")
    page = ctx.new_page()
    page.goto(f"http://localhost:{PORT}/")
    # The second poll brings the first pulse difference; catch a moment with a lit LED.
    # A flash lasts about 0.1 s, so poll every frame rather than waiting for a mutation.
    page.wait_for_function("document.querySelector('.led.on') !== null", polling="raf", timeout=10000)
    page.screenshot(path=str(path), full_page=full_page)
    ctx.close()
    print("wrote", path.relative_to(ROOT))


def social(browser):
    shot = base64.b64encode((OUT / "web-ui.png").read_bytes()).decode()
    html = f"""<!doctype html><meta charset="utf-8">
<style>
  body {{ margin: 0; width: 1280px; height: 640px; background: #f3f5f7; font-family: system-ui, sans-serif;
         display: flex; align-items: center; gap: 56px; padding: 0 0 0 96px; box-sizing: border-box; overflow: hidden; }}
  .text {{ display: flex; flex-direction: column; gap: 22px; width: 470px; flex: none; }}
  .brand {{ display: flex; align-items: center; gap: 22px; }}
  svg {{ width: 104px; height: 70px; color: #2563a8; }}
  h1 {{ margin: 0; font-size: 84px; letter-spacing: -.02em; color: #17202b; }}
  h1 b {{ color: #2563a8; }}
  p {{ margin: 0; font-size: 30px; line-height: 1.3; color: #3d4a58; }}
  .tags {{ font-size: 21px; color: #5d6b7a; }}
  img {{ height: 520px; border-radius: 14px; box-shadow: 0 20px 50px rgba(23, 32, 43, .18);
         border: 1px solid #dde2e8; object-fit: cover; object-position: left top; width: 900px; }}
</style>
<div class="text">
  <div class="brand"><svg viewBox="0 0 44 30" fill="none" stroke="currentColor" stroke-width="2.6"
    stroke-linecap="round" stroke-linejoin="round"><path d="M2 24h7V6h6v18h7V6h6v18h7V6h6"/></svg>
    <h1><b>s0</b>meter</h1></div>
  <p>S0 pulse counter for the Raspberry Pi: electricity, water and gas meters.</p>
  <span class="tags">MQTT · REST API · live web page</span>
</div>
<img src="data:image/png;base64,{shot}" alt="">"""
    page = browser.new_page(viewport={"width": 1280, "height": 640})
    page.set_content(html)
    path = ROOT / "docs/social-preview.png"
    page.screenshot(path=str(path))
    page.close()
    print("wrote", path.relative_to(ROOT))


def main():
    server = ThreadingHTTPServer(("localhost", PORT), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    OUT.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as p:
        browser = p.chromium.launch()
        shoot(browser, OUT / "web-ui.png", 1280, 440, "light")
        shoot(browser, OUT / "web-ui-dark.png", 1280, 440, "dark")
        # Phone: the first screen only, so it sits next to the desktop shot in the README.
        shoot(browser, OUT / "web-ui-phone.png", 390, 700, "light", scale=2, full_page=False)
        social(browser)
        browser.close()
    server.shutdown()


if __name__ == "__main__":
    main()
