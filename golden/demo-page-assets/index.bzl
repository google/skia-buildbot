"""This module defines the GOLD_DEMO_PAGE_ASSETS constant."""

# Any Gold demo pages that require the below static assets should pass this constant to their
# sk_demo_page_server rules via the static_assets argument.
GOLD_DEMO_PAGE_ASSETS = {
    "/img/diffs": [
        "//golden/demo-page-assets:2fa58aa430e9c815755624ca6cca4a72-ed4a8cf9ea9fbb57bf1f302537e07572.png",
        "//golden/demo-page-assets:5d8c80eda80e015d633a4125ab0232dc-fbd3de3fff6b852ae0bb6751b9763d27.png",
        "//golden/demo-page-assets:6246b773851984c726cb2e1cb13510c2-99c58c7002073346ff55f446d47d6311.png",
        "//golden/demo-page-assets:6246b773851984c726cb2e1cb13510c2-ec3b8f27397d99581e06eaa46d6d5837.png",
    ],
    "/img/images": [
        "//golden/demo-page-assets:2fa58aa430e9c815755624ca6cca4a72.png",
        "//golden/demo-page-assets:5d8c80eda80e015d633a4125ab0232dc.png",
        "//golden/demo-page-assets:6246b773851984c726cb2e1cb13510c2.png",
        "//golden/demo-page-assets:99c58c7002073346ff55f446d47d6311.png",
        "//golden/demo-page-assets:ec3b8f27397d99581e06eaa46d6d5837.png",
        "//golden/demo-page-assets:ed4a8cf9ea9fbb57bf1f302537e07572.png",
        "//golden/demo-page-assets:fbd3de3fff6b852ae0bb6751b9763d27.png",
    ],
}
