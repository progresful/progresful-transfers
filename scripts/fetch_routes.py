#!/usr/bin/env python3
"""Fetches the real road geometry between every two places of the map from the public OSRM
demo server and writes internal/ui/static/routes.json: {"a|b": {"line": <encoded polyline>,
"km": .., "min": ..}} with a < b. One request a second (the demo server's policy); a pair
that fails is left out and drawn as a straight line by the UI. Run once; the result is
committed with the UI.

  python3 -I scripts/fetch_routes.py
"""
import json, os, sys, time, urllib.request

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
OUT = os.path.join(ROOT, "internal", "ui", "static", "routes.json")
URL = "https://router.project-osrm.org/route/v1/driving/{},{};{},{}?overview=simplified&geometries=polyline"
PAUSE = 1.05

places = json.load(open(os.path.join(ROOT, "docs", "official", "map.json")))["places"]
routes = json.load(open(OUT)) if os.path.exists(OUT) else {}
pairs = [(a, b) for a in places for b in places if a["id"] < b["id"]]
for i, (a, b) in enumerate(pairs):
    key = f"{a['id']}|{b['id']}"
    if key in routes:
        continue
    try:
        req = urllib.request.Request(URL.format(a["lon"], a["lat"], b["lon"], b["lat"]), headers={"User-Agent": "route-fetch"})
        r = json.load(urllib.request.urlopen(req, timeout=20))["routes"][0]
        routes[key] = {"line": r["geometry"], "km": round(r["distance"] / 1000, 1), "min": round(r["duration"] / 60)}
    except Exception as err:  # noqa: BLE001 - any failure leaves the pair out
        print(f"{key}: {err}", file=sys.stderr)
    if i % 25 == 0:
        json.dump(routes, open(OUT, "w"))
        print(f"{len(routes)} of {len(pairs)}", flush=True)
    time.sleep(PAUSE)
json.dump(routes, open(OUT, "w"))
print(f"done: {len(routes)} of {len(pairs)} routes in {OUT}")
