"""Render WinGet and Homebrew manifests from the files in a release."""

import argparse
import hashlib
import json
from pathlib import Path
import re
from string import Template


PROJECT = Path(__file__).resolve().parents[1]
HOMEPAGE = "https://github.com/WeslyG/wireproxy-desktop"
TAG_PATTERN = r"v?(?:0|[1-9][0-9]*)(?:\.(?:0|[1-9][0-9]*)){2}"


def render(template, output, values):
    source = (PROJECT / "packaging" / template).read_text(encoding="utf-8")
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(Template(source).substitute(values), encoding="utf-8", newline="\n")


def render_release(tag, assets, output, license_name):
    if not re.fullmatch(TAG_PATTERN, tag):
        raise ValueError("expected a stable tag in the form X.Y.Z")
    if not license_name.strip() or "\n" in license_name or "\r" in license_name:
        raise ValueError("the WinGet license must be a non-empty single line")

    version = tag.removeprefix("v")
    values = {
        "VERSION": version,
        "TAG": tag,
        "HOMEPAGE": HOMEPAGE,
        "BASE_URL": f"{HOMEPAGE}/releases/download/{tag}",
        "LICENSE": json.dumps(license_name, ensure_ascii=False),
    }
    for key, filename in {
        "WINDOWS_AMD64": f"wireproxy-desktop-setup-windows-amd64-{tag}.exe",
        "WINDOWS_ARM64": f"wireproxy-desktop-setup-windows-arm64-{tag}.exe",
        "MACOS_AMD64": f"wireproxy-desktop-macos-amd64-{tag}.zip",
        "MACOS_ARM64": f"wireproxy-desktop-macos-arm64-{tag}.zip",
    }.items():
        with (assets / filename).open("rb") as artifact:
            if not artifact.peek(1):
                raise ValueError(f"empty release asset: {filename}")
            values[key] = hashlib.file_digest(artifact, "sha256").hexdigest()

    manifests = output / "manifests" / "w" / "WeslyG" / "WireproxyDesktop" / version
    for template, name in (
        ("version", "WeslyG.WireproxyDesktop.yaml"),
        ("installer", "WeslyG.WireproxyDesktop.installer.yaml"),
        ("defaultLocale", "WeslyG.WireproxyDesktop.locale.en-US.yaml"),
    ):
        render(f"winget/{template}.yaml.in", manifests / name, values)
    render("homebrew/wireproxy-desktop.rb.in", output / "Casks" / "wireproxy-desktop.rb", values)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tag")
    parser.add_argument("assets", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--license", default="Unspecified")
    args = parser.parse_args()
    try:
        render_release(args.tag, args.assets, args.output, args.license)
    except (OSError, ValueError) as error:
        parser.exit(1, f"{error}\n")


if __name__ == "__main__":
    main()
