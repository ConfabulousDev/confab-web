#!/usr/bin/env bash


# ================================================
# Neon Quotas (0.25 CU)
# ================================================
# active_time_seconds:  633,600    (~22 business days)
# compute_time_seconds: 158,400    (~44 hours)
# written_data_bytes:   1,000,000,000  (~1 GB)
# data_transfer_bytes:  500,000,000    (~500 MB)
# logical_size_bytes:   100,000,000    (~100 MiB)
# ================================================

curl --request POST \
  --url https://console.neon.tech/api/v2/projects \
  --header 'Accept: application/json' \
  --header "Authorization: Bearer $NEON_API_KEY" \
  --header 'Content-Type: application/json' \
  --data '{
    "project": {
      "settings": {
        "quota": {
          "active_time_seconds": 633600,
          "compute_time_seconds": 158400,
          "written_data_bytes": 1000000000,
          "data_transfer_bytes": 500000000,
          "logical_size_bytes": 100000000
        }
      }
    }
  }' | jq
