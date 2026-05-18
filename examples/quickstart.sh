#!/usr/bin/env bash
# Mirror of the demo transcript from the v0.1.0 release plan.
#
# Run after `aegean login` has stored credentials.
set -euo pipefail

aegean domains add acme.com
aegean domains verify acme.com   # blocks until DKIM/SPF/DMARC pass

aegean keys create "Production app" --save
# -> aegean_sk_********** stored to ~/.aegean/credentials

aegean send \
    --to "ada@example.com" \
    --from "hello@acme.com" \
    --subject "Welcome, Ada" \
    --html "<h1>Hi Ada</h1><p>Thanks for joining.</p>"
# -> message id: 4b9b441c…
# -> status:     sent
