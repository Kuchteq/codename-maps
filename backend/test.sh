#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8000}"

echo "--- valid edit: Helsinki city centre → harbour ---"
curl -s -w "\nHTTP %{http_code}\n" -X POST "$BASE_URL/v1/edit" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Helsinki centre to harbour",
    "author": "tester",
    "prompt": "Add a pedestrian path from city centre to the harbour",
      "start": {
        "type": "Point",
        "coordinates": [24.9705, 60.1865]
      },
      "end": {
        "type": "Point",
        "coordinates": [24.9900, 60.1750]
      }
  }'

